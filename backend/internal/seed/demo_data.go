package seed

// demoProduct describes one example product. Variants are every colour × size combination.
type demoProduct struct {
	Slug        string
	Name        string
	Description string
	Category    string // category slug
	Premium     bool
	Material    string
	Badges      []string
	GiftBox     bool
	Preorder    bool
	SKU         string // SKU prefix, e.g. "JG-RIB"

	Colours []string // colour names from the colours table; empty = no colour choice
	SizeSet string   // size set name; "" = one size
	Sizes   []string // subset of the size set; nil = every size in the set

	PriceKobo   int64
	SizePrices  map[string]int64 // price per size where it differs from PriceKobo
	WasKobo     int64            // old price when on sale; 0 = not on sale
	SoldOut     []string         // "Colour/Size" (or "Colour", or "/Size") options with no stock
	LowStock    []string         // options with only 2 left, to show "Only 2 left"
	Details     [][2]string      // label, value; labels must exist in the category's fields
	CompleteWith []string        // slugs shown under "Complete the look"
}

const naira = 100 // kobo per naira

var demoProducts = []demoProduct{
	{
		Slug: "ribbed-lounge-joggers", Name: "Ribbed lounge joggers", Category: "joggers",
		Description: "Soft ribbed joggers with a high, wide waistband and a relaxed leg. Pairs with the matching cami top.",
		Material:    "Cotton-blend rib", Badges: []string{"New"}, GiftBox: true, SKU: "JG-RIB",
		Colours: []string{"Espresso", "Blush", "Black", "Grey marl"}, SizeSet: "Clothing sizes (UK)",
		PriceKobo: 14500 * naira, SoldOut: []string{"Blush/L"}, LowStock: []string{"Espresso/M"},
		Details: [][2]string{{"Fabric", "95% cotton, 5% elastane rib"}, {"Fit", "Relaxed, high waist"}, {"Inside leg (size M)", "74 cm"}, {"Model wears", "Size S · 168 cm tall"}},
		CompleteWith: []string{"square-neck-cami-top", "gold-huggie-hoops", "mini-structured-tote"},
	},
	{
		Slug: "cropped-baby-tee", Name: "Cropped baby tee", Category: "tops",
		Description: "A fitted cropped tee in soft cotton jersey. Sits just above the waistband of our joggers.",
		Material:    "Cotton jersey", Preorder: true, SKU: "TP-BABY",
		Colours: []string{"Blush", "Black", "Ivory"}, SizeSet: "Clothing sizes (UK)",
		PriceKobo: 9000 * naira, SoldOut: []string{"Blush", "Black", "Ivory"},
		Details: [][2]string{{"Fabric", "100% cotton"}, {"Fit", "Fitted, cropped"}, {"Model wears", "Size S · 168 cm tall"}},
	},
	{
		Slug: "ribbed-jogger-co-ord-set", Name: "Ribbed jogger co-ord set", Category: "co-ord-sets",
		Description: "Our ribbed joggers with the matching long-sleeve top, sold together.",
		Material:    "Cotton-blend rib", Badges: []string{"New"}, GiftBox: true, SKU: "CO-RIB",
		Colours: []string{"Espresso", "Blush"}, SizeSet: "Clothing sizes (UK)", Sizes: []string{"S", "M", "L"},
		PriceKobo: 24000 * naira,
		Details: [][2]string{{"Fabric", "95% cotton, 5% elastane rib"}, {"Fit", "Relaxed"}},
		CompleteWith: []string{"gold-mesh-ladies-watch"},
	},
	{
		Slug: "square-neck-cami-top", Name: "Square-neck cami top", Category: "tops",
		Description: "A ribbed cami with a square neckline and adjustable straps.",
		Material:    "Cotton-blend rib", SKU: "TP-CAMI",
		Colours: []string{"Espresso", "Blush", "Black"}, SizeSet: "Clothing sizes (UK)",
		PriceKobo: 8500 * naira,
		Details: [][2]string{{"Fabric", "95% cotton, 5% elastane rib"}, {"Fit", "Fitted"}},
		CompleteWith: []string{"ribbed-lounge-joggers"},
	},
	{
		Slug: "pearl-drop-earrings", Name: "Pearl drop earrings", Category: "earrings",
		Description: "A freshwater pearl hanging from a small hoop. Light enough for all-day wear.",
		Material:    "18k gold plated · freshwater pearl", GiftBox: true, SKU: "ER-PRL",
		Colours: []string{"Gold", "Silver"},
		PriceKobo: 12000 * naira,
		Details: [][2]string{{"Metal", "Stainless steel"}, {"Plating", "18k gold"}, {"Stone", "Freshwater pearl"}, {"Hypoallergenic", "Yes"}},
		CompleteWith: []string{"tennis-bracelet-18k"},
	},
	{
		Slug: "gold-huggie-hoops", Name: "Gold huggie hoops", Category: "earrings",
		Description: "Small chunky hoops that hug the earlobe.",
		Material:    "18k gold plated", SKU: "ER-HUG",
		Colours: []string{"Gold"},
		PriceKobo: 11000 * naira, LowStock: []string{"Gold"},
		Details: [][2]string{{"Metal", "Stainless steel"}, {"Plating", "18k gold"}, {"Hypoallergenic", "Yes"}},
	},
	{
		Slug: "tennis-bracelet-18k", Name: "18k plated tennis bracelet", Category: "bracelets", Premium: true,
		Description: "A single line of cubic zirconia stones set in 18k gold plating.",
		Material:    "18k gold plated · cubic zirconia", GiftBox: true, SKU: "BR-TEN",
		Colours: []string{"Gold"}, SizeSet: "Bracelet sizes",
		PriceKobo: 95000 * naira, LowStock: []string{"Gold/S"},
		Details: [][2]string{{"Metal", "Brass"}, {"Plating", "18k gold"}, {"Stone", "Cubic zirconia"}, {"Hypoallergenic", "Yes"}},
		CompleteWith: []string{"pearl-drop-earrings", "solitaire-ring"},
	},
	{
		Slug: "solitaire-ring", Name: "Solitaire ring", Category: "rings", Premium: true,
		Description: "A single cubic zirconia stone on a slim 18k gold-plated band.",
		Material:    "18k gold plated · cubic zirconia", GiftBox: true, SKU: "RG-SOL",
		Colours: []string{"Gold", "Silver", "Rose gold"}, SizeSet: "Ring sizes (US)",
		PriceKobo: 48000 * naira,
		Details: [][2]string{{"Metal", "Brass"}, {"Plating", "18k gold"}, {"Stone", "Cubic zirconia"}, {"Hypoallergenic", "Yes"}},
	},
	{
		Slug: "beaded-stretch-bracelet", Name: "Beaded stretch bracelet", Category: "bead-pieces",
		Description: "Gold-tone beads on a strong stretch cord. Stack two or three.",
		Material:    "Glass and gold-tone beads", SKU: "BD-STR",
		Colours: []string{"Gold", "Ivory"}, SizeSet: "Bracelet sizes",
		PriceKobo: 7500 * naira, WasKobo: 9500 * naira,
		Details: [][2]string{{"Metal", "Gold-tone brass"}, {"Hypoallergenic", "Yes"}},
	},
	{
		Slug: "gold-mesh-ladies-watch", Name: "Gold mesh ladies’ watch", Category: "watches", Premium: true,
		Description: "A slim gold-tone case on a fine stainless mesh strap.",
		Material:    "Stainless steel", Badges: []string{"Bestseller"}, GiftBox: true, SKU: "WT-MSH",
		Colours: []string{"Gold", "Silver", "Rose gold"},
		PriceKobo: 64000 * naira,
		Details: [][2]string{{"Case size", "28 mm"}, {"Strap", "Stainless steel mesh"}, {"Movement", "Quartz"}, {"Water resistance", "3 ATM (splash proof)"}, {"Warranty", "6 months"}, {"In the box", "Gift box and spare battery"}},
		CompleteWith: []string{"tennis-bracelet-18k"},
	},
	{
		Slug: "mini-structured-tote", Name: "Mini structured tote", Category: "handbags-and-totes",
		Description: "A compact structured tote with a detachable crossbody strap. Fits your phone, wallet and keys.",
		Material:    "Faux leather", GiftBox: true, SKU: "BG-TOT",
		Colours: []string{"Tan", "Black", "Cream", "Blush"},
		PriceKobo: 32000 * naira, LowStock: []string{"Cream"},
		Details: [][2]string{{"Dimensions (W × H × D)", "24 × 18 × 10 cm"}, {"Strap drop", "Handle 10 cm · strap 55 cm, detachable"}, {"Material", "Faux leather, suede-feel lining"}, {"Closure", "Magnetic snap"}, {"Inside", "1 zip pocket, 2 slip pockets"}, {"Fits", "Phone up to 6.7\", wallet, keys"}},
		CompleteWith: []string{"gold-huggie-hoops"},
	},
	{
		Slug: "leather-evening-clutch", Name: "Leather evening clutch", Category: "clutches-and-evening", Premium: true,
		Description: "A soft leather clutch with a gold chain you can tuck inside.",
		Material:    "Genuine leather", GiftBox: true, SKU: "BG-CLU",
		Colours: []string{"Black", "Tan"},
		PriceKobo: 78000 * naira, LowStock: []string{"Tan"},
		Details: [][2]string{{"Dimensions (W × H × D)", "26 × 14 × 4 cm"}, {"Strap drop", "Chain 50 cm, removable"}, {"Material", "Genuine leather"}, {"Closure", "Zip"}, {"Fits", "Phone, cards, lipstick"}},
	},
	{
		Slug: "opaque-black-tights", Name: "Opaque black tights", Category: "tights",
		Description: "Soft, opaque 40 denier tights with a comfort waistband.",
		Material:    "40 denier", SKU: "TT-OPQ",
		Colours: []string{"Black"}, SizeSet: "Tights sizes",
		PriceKobo: 3500 * naira, SizePrices: map[string]int64{"XL": 4200 * naira, "XXL": 4200 * naira},
		Details: [][2]string{{"Fabric", "88% nylon, 12% elastane"}, {"Denier", "40"}, {"Pack size", "1 pair"}, {"Washing", "Hand wash cold"}},
	},
	{
		Slug: "seamless-cotton-panties-3-pack", Name: "Seamless cotton panties (3-pack)", Category: "panties",
		Description: "Three seamless briefs with a soft cotton gusset. No lines under clothes.",
		Material:    "Cotton and modal", SKU: "PN-SML",
		Colours: []string{"Black", "Nude 2"}, SizeSet: "Panty sizes",
		PriceKobo: 6500 * naira,
		Details: [][2]string{{"Fabric", "Modal with cotton gusset"}, {"Pack size", "3"}, {"Washing", "Machine wash 30°C"}},
	},
}
