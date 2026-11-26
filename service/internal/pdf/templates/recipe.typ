// One recipe: the cookbook page (A4) or the card (A5). Design in lib.typ.
#import "lib.typ": recipe, setup
#let data = json("data.json")
#let r = data.recipes.at(0)
#set document(title: r.title)
#show: setup.with(data)

#recipe(data, 0, r)
