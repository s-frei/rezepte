package shopping

import (
	"slices"
	"strings"
)

// Department is a store section an item is sorted under.
type Department string

// The departments an item can be sorted under.
const (
	Produce   Department = "produce"
	Bakery    Department = "bakery"
	Chilled   Department = "chilled"
	Meat      Department = "meat"
	Pantry    Department = "pantry"
	Frozen    Department = "frozen"
	Drinks    Department = "drinks"
	Household Department = "household"
	Other     Department = "other"
)

// Departments lists every department in display order.
var Departments = []Department{Produce, Bakery, Chilled, Meat, Pantry, Frozen, Drinks, Household, Other}

// Valid reports whether d is one of Departments.
func (d Department) Valid() bool { return slices.Contains(Departments, d) }

// words holds common German and English grocery words per department.
var words = map[Department]string{
	Produce: "zwiebel, knoblauch, kartoffel, tomate, gurke, paprika, karotte, möhre, zitrone, limette, apfel, birne, banane, orange, " +
		"salat, spinat, brokkoli, blumenkohl, zucchini, aubergine, pilz, champignon, lauch, sellerie, petersilie, basilikum, ingwer, " +
		"avocado, beere, erdbeere, traube, kürbis, radieschen, kohl, fenchel, schalotte, " +
		"onion, garlic, potato, tomato, cucumber, bell pepper, carrot, lemon, lime, apple, pear, banana, lettuce, spinach, broccoli, " +
		"schote, cauliflower, eggplant, mushroom, leek, celery, parsley, basil, ginger, dill, thyme, rosemary, chive, schnittlauch, thymian, rosmarin, minze, mint, pumpkin, cabbage, shallot, herb, scallion, cilantro",
	Bakery: "brot, toastbrot, brötchen, baguette, croissant, brezel, tortilla, " +
		"bread, bun, roll, bagel, pita",
	Chilled: "milch, sahne, butter, joghurt, quark, käse, mozzarella, parmesan, frischkäse, schmand, ei, eier, margarine, skyr, " +
		"milk, cream, heavy cream, yogurt, cheese, cream cheese, egg, sour cream, feta, tofu, " +
		"mascarpone, crème fraîche, schlagsahne, buttermilch, cheddar, gouda, emmentaler, ricotta, butterschmalz, schmalz, lard, pudding, sauerrahm, eigelb, eiweiß, egg yolk, egg white, puff pastry, blätterteig, hefeteig",
	Meat: "fleisch, hackfleisch, hähnchen, huhn, rind, schwein, schinken, speck, wurst, salami, lachs, fisch, thunfisch, garnele, steak, " +
		"beef, pork, chicken, ham, bacon, sausage, salmon, fish, shrimp, turkey, lamb, mince, " +
		"schnitzel, braten, roulade, kotelett, filet, cutlet, chop, brät, sausage meat, meat, bratwurst",
	Pantry: "mehl, zucker, salz, öl, olivenöl, essig, nudel, pasta, reis, haferflocke, kapern, bohne, linse, kichererbse, honig, senf, " +
		"lasagneplatte, ketchup, gewürz, zimt, backpulver, hefe, schokolade, nuss, mandel, brühe, passierte tomaten, dosentomaten, " +
		"flour, sugar, salt, oil, olive oil, vinegar, noodle, rice, oat, caper, bean, lentil, chickpea, honey, mustard, spice, " +
		"cinnamon, baking powder, yeast, chocolate, nut, almond, broth, stock, cereal, " +
		"kokosmilch, coconut milk, pfeffer, pepper, black pepper, pfefferkörner, paprikapulver, curry powder, mustard powder, powder, pulver, muskat, kümmel, lorbeerblätter, bay leaf, bay leaves, nelke, vanille, vanilleschote, vanilla, speisestärke, cornstarch, tomatenmark, tomato puree, tomato paste, semmelbrösel, paniermehl, brösel, breadcrumb, sauce, soße, mayonnaise, mayo, chutney, macaroni, makkaroni, spaghetti, rosine, sultana, date, dattel, sage, salbei, oregano, worcestershire sauce",
	Frozen: "tiefkühl, eis, pommes, tk-gemüse, frozen, ice cream, fries",
	Drinks: "wasser, saft, wein, bier, cola, limonade, kaffee, tee, sekt, " +
		"water, juice, wine, beer, soda, coffee, tea, ale, brown ale",
	Household: "spülmittel, toilettenpapier, küchenrolle, müllbeutel, waschmittel, seife, schwamm, spülmaschinentabs, " +
		"dish soap, toilet paper, paper towel, trash bag, detergent, soap, sponge, foil, alufolie, backpapier",
}

var defaults = func() map[string]Department {
	m := map[string]Department{}
	for dep, list := range words {
		for _, w := range strings.Split(list, ",") {
			m[NameKey(w)] = dep
		}
	}
	return m
}()

// DefaultDepartment returns the built-in department for a NameKey, or Other.
// An exact match wins; otherwise the longest known word the key ends with
// does ("rinderhackfleisch" ends with "hackfleisch", "red bell pepper" with
// "bell pepper"). A suffix needs four letters, or a word boundary before it
// ("vegetable oil" ends with " oil"), so "graham" never matches "ham"; "öl"
// ("rapsöl") is the one short compound ending that is safe. A qualifier after
// a comma or in parentheses is ignored ("hackfleisch, gemischt (halb und halb)").
func DefaultDepartment(key string) Department {
	if i := strings.IndexAny(key, ",("); i > 0 {
		key = NameKey(key[:i])
	}
	if d, ok := defaults[key]; ok {
		return d
	}
	best, bestLen := Other, 0
	for w, d := range defaults {
		if len(w) <= bestLen || !strings.HasSuffix(key, w) {
			continue
		}
		if len(w) >= 4 || w == "öl" || strings.HasSuffix(key, " "+w) {
			best, bestLen = d, len(w)
		}
	}
	return best
}
