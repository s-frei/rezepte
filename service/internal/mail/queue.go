package mail

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/textproto"
	"slices"
	"strings"
	"time"

	"github.com/s-frei/rezepte/service/internal/db"
	"github.com/s-frei/rezepte/service/internal/db/sqlc"
)

// queueSize is how many mails may wait for the worker before a sender
// waits for room; a daily digest to a large household fits.
const queueSize = 1024

// retryAfter is when a mail whose send failed for a reason that may pass
// (a broken or silent connection, a 4xx reply) is tried again, counted from the
// failure: four attempts over about six minutes. A 5xx reply is final.
var retryAfter = []time.Duration{15 * time.Second, time.Minute, 5 * time.Minute}

// logKeep is how long the mail history keeps a row.
const logKeep = 30 * 24 * time.Hour

// callbackTimeout bounds a mail's OnFail, which runs on the worker.
const callbackTimeout = 10 * time.Second

// errInterrupted is the history's reason for a mail still waiting when
// Rezepte stopped: the queue lives in memory, so it never went out. Its
// text is a code the outbox translates, like "disabled" (see Detail).
var errInterrupted = errors.New("interrupted")

// job is one mail on its way: its history row, the message and what to do
// once it went out or finally failed.
type job struct {
	id       int64
	env      Envelope
	what     string
	onSent   func(context.Context)
	onFail   func(context.Context, error)
	attempts int
	due      time.Time
	// lastErr is the latest failed attempt's, which the history keeps when
	// a retry then gets through.
	lastErr error
}

// Start hands every later send to a worker in its own goroutine and makes
// the senders return at once: the worker sends what waits over one
// connection and retries what failed for a passing reason. A mail still
// queued from a run that ended is marked failed first. Without Start, as
// in the command line and in tests, every send happens inside its call.
func (s *Service) Start(ctx context.Context) {
	now := db.FormatTime(s.now())
	if err := s.q.InterruptQueuedMailLog(ctx, sqlc.InterruptQueuedMailLogParams{Error: errInterrupted.Error(), FinishedAt: &now}); err != nil {
		s.logger.Warn("mail: history cleanup failed", "err", err)
	}
	s.queue, s.quit, s.done, s.stopping = make(chan *job, queueSize), make(chan struct{}), make(chan struct{}), make(chan struct{})
	go s.work(context.WithoutCancel(ctx))
}

// Stop sends what is queued and waits for it at most wait; a retry still
// waiting is given up and marked in the history, and a mail handed in from
// now on fails at once. A second call does nothing.
func (s *Service) Stop(wait time.Duration) {
	if s.queue == nil {
		return
	}
	// Wakes a sender waiting on a full queue, which holds the read lock.
	select {
	case <-s.stopping:
	default:
		close(s.stopping)
	}
	s.stopMu.Lock()
	again := s.stopped
	s.stopped = true
	s.stopMu.Unlock()
	if again {
		return
	}
	close(s.quit)
	select {
	case <-s.done:
	case <-time.After(wait):
		s.logger.Warn("mail: still sending at exit, the rest is marked as not sent at the next start")
	}
}

// enqueue hands j to the worker, or fails it once Stop began. It fails j
// too when ctx ends while the queue is full, so a sender never waits
// longer than its own context allows. The read lock keeps Stop from
// closing the queue between the check and the hand-over.
func (s *Service) enqueue(ctx context.Context, j *job) {
	s.stopMu.RLock()
	defer s.stopMu.RUnlock()
	if s.stopped {
		s.finish(ctx, j, errInterrupted)
		return
	}
	select {
	case s.queue <- j:
	case <-s.stopping:
		s.finish(ctx, j, errInterrupted)
	case <-ctx.Done():
		s.finish(ctx, j, fmt.Errorf("mail queue full: %w", ctx.Err()))
	}
}

func (s *Service) work(ctx context.Context) {
	defer close(s.done)
	var waiting []*job
	prune := time.NewTicker(time.Hour)
	defer prune.Stop()
	s.prune(ctx)
	for {
		var retry <-chan time.Time
		var timer *time.Timer
		if len(waiting) > 0 {
			timer = time.NewTimer(time.Until(waiting[0].due))
			retry = timer.C
		}
		select {
		case j := <-s.queue:
			waiting = s.run(ctx, []*job{j}, waiting)
		case <-retry:
			now := s.now()
			n := 0
			for n < len(waiting) && !waiting[n].due.After(now) {
				n++
			}
			waiting = s.run(ctx, slices.Clone(waiting[:n]), waiting[n:])
		case <-prune.C:
			s.prune(ctx)
		case <-s.quit:
			// run takes everything queued meanwhile; what waits for a retry
			// is given up.
			for _, j := range s.run(ctx, nil, waiting) {
				s.finish(ctx, j, errInterrupted)
			}
			return
		}
		if timer != nil {
			timer.Stop()
		}
	}
}

// run sends batch, and every mail queued meanwhile, over one session, and
// returns waiting with the mails to retry added in the order they are due.
// A failed connection fails the rest of the batch with it instead of
// connecting again for each.
func (s *Service) run(ctx context.Context, batch, waiting []*job) []*job {
	cfg, cfgErr := s.sendable(ctx)
	var sess *session
	var openErr error
	defer func() {
		if sess != nil {
			sess.quit()
			sess.close()
		}
	}()
	for {
		var j *job
		if len(batch) > 0 {
			j, batch = batch[0], batch[1:]
		} else {
			select {
			case j = <-s.queue:
			default:
			}
		}
		if j == nil {
			return waiting
		}
		j.attempts++
		err := cfgErr
		if err == nil {
			err = openErr
		}
		if err == nil && sess == nil {
			if sess, err = open(ctx, cfg); err != nil {
				openErr = err
			}
		}
		if err == nil {
			if err = sess.send(j.env); err != nil {
				sess.close()
				sess = nil
			}
		}
		if err != nil && !errors.Is(err, ErrDisabled) {
			err = fmt.Errorf("%w: %w", ErrSend, err)
		}
		if err != nil && retryable(err) && j.attempts <= len(retryAfter) {
			j.due = s.now().Add(retryAfter[j.attempts-1])
			j.lastErr = err
			s.markRetry(ctx, j)
			s.logger.Info("mail: "+j.what+" not sent yet, trying again", "domain", domainOf(j.env.To), "attempt", j.attempts, "reason", reasonOf(err))
			i, _ := slices.BinarySearchFunc(waiting, j, func(a, b *job) int { return a.due.Compare(b.due) })
			waiting = slices.Insert(waiting, i, j)
			continue
		}
		s.finish(ctx, j, err)
	}
}

// sendable is the configuration a send uses now, ErrDisabled when mail is
// off.
func (s *Service) sendable(ctx context.Context) (Config, error) {
	cfg, src, err := s.Effective(ctx)
	if err != nil {
		return Config{}, err
	}
	if src == SourceNone || s.publicURL == "" {
		return Config{}, ErrDisabled
	}
	return cfg, nil
}

// retryable reports whether err may pass on its own: a 4xx reply, or a
// connection that broke or went silent. Everything else - a 5xx reply, a
// certificate, a TLS or sign-in mismatch, mail switched off - stays until
// somebody changes something, and a connection lost after the message was
// handed over is not retried either: the server may have accepted it, and
// a second setup mail would carry a second link.
func retryable(err error) bool {
	if errors.Is(err, errUnconfirmed) {
		return false
	}
	if te, ok := errors.AsType[*textproto.Error](err); ok {
		return te.Code/100 == 4
	}
	// A host name that does not exist stays a typo; a refused connection
	// may be a server restarting, and is retried.
	if de, ok := errors.AsType[*net.DNSError](err); ok && de.IsNotFound {
		return false
	}
	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

// record writes env's history row, queued.
func (s *Service) record(ctx context.Context, env Envelope) (int64, error) {
	id, err := s.q.InsertMailLog(ctx, sqlc.InsertMailLogParams{
		Kind: env.Kind, Recipient: env.To, Subject: env.Subject, CreatedAt: db.FormatTime(s.now()),
	})
	if err != nil {
		return 0, fmt.Errorf("record mail: %w", err)
	}
	return id, nil
}

// finish settles j: its history row, the log line, the reporter and j's
// onSent or onFail.
func (s *Service) finish(ctx context.Context, j *job, err error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), callbackTimeout)
	defer cancel()
	now := db.FormatTime(s.now())
	status, detail := "sent", ""
	if err != nil {
		status, detail = "failed", Detail(err)
	} else if j.lastErr != nil {
		detail = Detail(j.lastErr)
	}
	if j.id != 0 {
		if derr := s.q.FinishMailLog(ctx, sqlc.FinishMailLogParams{
			Status: status, Attempts: int64(j.attempts), Error: detail, FinishedAt: &now, ID: j.id,
		}); derr != nil {
			s.logger.Warn("mail: history update failed", "err", derr)
		}
	}
	if err == nil {
		if s.reporter != nil {
			s.reporter.MailReset(ctx)
		}
		if j.onSent != nil {
			j.onSent(ctx)
		}
		return
	}
	// The error text can quote the address and a link is never logged: the
	// recipient's domain and a server's reply code only.
	s.logger.Warn("mail: "+j.what+" not sent", "domain", domainOf(j.env.To), "reason", reasonOf(err), "attempts", j.attempts)
	if se, ok := errors.AsType[*setupError](err); ok && s.reporter != nil {
		s.reporter.MailFailed(ctx, se.Reason)
	}
	if j.onFail != nil {
		j.onFail(ctx, err)
	}
}

func reasonOf(err error) any {
	if reply, ok := errors.AsType[*textproto.Error](err); ok {
		return reply.Code
	}
	if errors.Is(err, errInterrupted) {
		return "interrupted"
	}
	return "send failed"
}

// Detail is err as the owner reads it: without the "send failed" mark, and
// with a server's reply unquoted, as "535 5.7.8 Authentication failed".
// Rezepte's own two reasons are codes the outbox translates: "interrupted"
// and "disabled".
func Detail(err error) string {
	switch {
	case errors.Is(err, errInterrupted):
		return "interrupted"
	case errors.Is(err, ErrDisabled):
		return "disabled"
	}
	detail := strings.TrimPrefix(err.Error(), ErrSend.Error()+": ")
	if te, ok := errors.AsType[*textproto.Error](err); ok {
		detail = strings.Replace(detail, te.Error(), fmt.Sprintf("%d %s", te.Code, te.Msg), 1)
	}
	return detail
}

// markRetry records in j's history row that an attempt failed and when the
// next one is due.
func (s *Service) markRetry(ctx context.Context, j *job) {
	next := db.FormatTime(j.due)
	if err := s.q.RetryMailLog(ctx, sqlc.RetryMailLogParams{
		Attempts: int64(j.attempts), Error: Detail(j.lastErr), NextAttemptAt: &next, ID: j.id,
	}); err != nil {
		s.logger.Warn("mail: history update failed", "err", err)
	}
}

// prune drops history rows older than logKeep.
func (s *Service) prune(ctx context.Context) {
	if err := s.q.PruneMailLog(ctx, db.FormatTime(s.now().Add(-logKeep))); err != nil {
		s.logger.Warn("mail: history prune failed", "err", err)
	}
}

// Sent is one row of the mail history. Error is the last failed attempt's
// reason, also on a mail a retry got through; NextAttemptAt is set while a
// retry waits.
type Sent struct {
	ID            int64
	Kind          string
	To            string
	Subject       string
	Status        string
	Attempts      int
	Error         string
	CreatedAt     time.Time
	FinishedAt    *time.Time
	NextAttemptAt *time.Time
}

// Outbox is one page of the mail history and the counts over all of it.
type Outbox struct {
	Items                 []Sent
	Total, Failed, Queued int
	// More is set when older rows follow the page.
	More bool
}

// historyPage is how many mails one page of History holds.
const historyPage = 50

// History lists the mails older than before (0: the newest), newest first,
// a page at a time; a status ("queued", "sent", "failed") lists only those.
func (s *Service) History(ctx context.Context, before int64, status string) (Outbox, error) {
	if before <= 0 {
		before = math.MaxInt64
	}
	rows, err := s.q.ListMailLog(ctx, sqlc.ListMailLogParams{Before: before, Status: status, Lim: historyPage + 1})
	if err != nil {
		return Outbox{}, fmt.Errorf("list mail history: %w", err)
	}
	counts, err := s.q.CountMailLog(ctx)
	if err != nil {
		return Outbox{}, fmt.Errorf("count mail history: %w", err)
	}
	out := Outbox{Total: int(counts.Total), Failed: int(counts.Failed), Queued: int(counts.Queued), More: len(rows) > historyPage}
	rows = rows[:min(len(rows), historyPage)]
	out.Items = make([]Sent, len(rows))
	for i, r := range rows {
		m := Sent{ID: r.ID, Kind: r.Kind, To: r.Recipient, Subject: r.Subject, Status: r.Status,
			Attempts: int(r.Attempts), Error: r.Error}
		if m.CreatedAt, err = db.ParseTime(r.CreatedAt); err != nil {
			return Outbox{}, err
		}
		if m.FinishedAt, err = parseOptional(r.FinishedAt); err != nil {
			return Outbox{}, err
		}
		if m.NextAttemptAt, err = parseOptional(r.NextAttemptAt); err != nil {
			return Outbox{}, err
		}
		out.Items[i] = m
	}
	return out, nil
}

func parseOptional(v *string) (*time.Time, error) {
	if v == nil {
		return nil, nil
	}
	t, err := db.ParseTime(*v)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
