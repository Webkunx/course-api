package main

// Synthetic Spanish-English vocabulary and templates. Not linguistically
// accurate — purely shaped to produce plausible-looking course content at
// scale. All output is deterministic from the seed passed to the RNG.

type wordPair struct {
	ES string
	EN string
}

var nouns = []wordPair{
	{"casa", "house"}, {"perro", "dog"}, {"gato", "cat"}, {"libro", "book"},
	{"mesa", "table"}, {"silla", "chair"}, {"ventana", "window"}, {"puerta", "door"},
	{"agua", "water"}, {"comida", "food"}, {"escuela", "school"}, {"trabajo", "work"},
	{"ciudad", "city"}, {"pueblo", "town"}, {"calle", "street"}, {"parque", "park"},
	{"playa", "beach"}, {"montaña", "mountain"}, {"río", "river"}, {"bosque", "forest"},
	{"sol", "sun"}, {"luna", "moon"}, {"estrella", "star"}, {"nube", "cloud"},
	{"lluvia", "rain"}, {"viento", "wind"}, {"nieve", "snow"}, {"tormenta", "storm"},
	{"amigo", "friend"}, {"familia", "family"}, {"madre", "mother"}, {"padre", "father"},
	{"hermano", "brother"}, {"hermana", "sister"}, {"hijo", "son"}, {"hija", "daughter"},
	{"profesor", "teacher"}, {"estudiante", "student"}, {"médico", "doctor"}, {"abogado", "lawyer"},
	{"coche", "car"}, {"tren", "train"}, {"avión", "plane"}, {"barco", "boat"},
	{"manzana", "apple"}, {"naranja", "orange"}, {"plátano", "banana"}, {"uva", "grape"},
	{"pan", "bread"}, {"queso", "cheese"}, {"leche", "milk"}, {"café", "coffee"},
	{"té", "tea"}, {"vino", "wine"}, {"cerveza", "beer"}, {"jugo", "juice"},
	{"camino", "path"}, {"camino", "road"}, {"jardín", "garden"}, {"flor", "flower"},
	{"árbol", "tree"}, {"hoja", "leaf"}, {"piedra", "stone"}, {"arena", "sand"},
	{"fuego", "fire"}, {"hielo", "ice"}, {"luz", "light"}, {"sombra", "shadow"},
	{"corazón", "heart"}, {"mente", "mind"}, {"alma", "soul"}, {"sueño", "dream"},
	{"voz", "voice"}, {"palabra", "word"}, {"historia", "story"}, {"canción", "song"},
	{"música", "music"}, {"baile", "dance"}, {"fiesta", "party"}, {"cena", "dinner"},
	{"desayuno", "breakfast"}, {"almuerzo", "lunch"}, {"plato", "plate"}, {"taza", "cup"},
	{"reloj", "clock"}, {"tiempo", "time"}, {"año", "year"}, {"mes", "month"},
	{"semana", "week"}, {"día", "day"}, {"hora", "hour"}, {"minuto", "minute"},
}

var verbs = []wordPair{
	{"hablar", "to speak"}, {"comer", "to eat"}, {"beber", "to drink"}, {"vivir", "to live"},
	{"correr", "to run"}, {"caminar", "to walk"}, {"saltar", "to jump"}, {"nadar", "to swim"},
	{"leer", "to read"}, {"escribir", "to write"}, {"escuchar", "to listen"}, {"mirar", "to look"},
	{"trabajar", "to work"}, {"estudiar", "to study"}, {"aprender", "to learn"}, {"enseñar", "to teach"},
	{"comprar", "to buy"}, {"vender", "to sell"}, {"pagar", "to pay"}, {"cobrar", "to charge"},
	{"abrir", "to open"}, {"cerrar", "to close"}, {"entrar", "to enter"}, {"salir", "to leave"},
	{"comenzar", "to begin"}, {"terminar", "to finish"}, {"continuar", "to continue"}, {"parar", "to stop"},
	{"amar", "to love"}, {"odiar", "to hate"}, {"querer", "to want"}, {"necesitar", "to need"},
	{"dar", "to give"}, {"tomar", "to take"}, {"recibir", "to receive"}, {"enviar", "to send"},
	{"ver", "to see"}, {"oír", "to hear"}, {"sentir", "to feel"}, {"pensar", "to think"},
	{"creer", "to believe"}, {"saber", "to know"}, {"conocer", "to know (a person)"}, {"recordar", "to remember"},
	{"olvidar", "to forget"}, {"perder", "to lose"}, {"encontrar", "to find"}, {"buscar", "to search"},
	{"jugar", "to play"}, {"ganar", "to win"}, {"perder", "to lose"}, {"empatar", "to tie"},
	{"viajar", "to travel"}, {"volar", "to fly"}, {"conducir", "to drive"}, {"caminar", "to walk"},
}

var adjectives = []wordPair{
	{"grande", "big"}, {"pequeño", "small"}, {"alto", "tall"}, {"bajo", "short"},
	{"rápido", "fast"}, {"lento", "slow"}, {"fuerte", "strong"}, {"débil", "weak"},
	{"caliente", "hot"}, {"frío", "cold"}, {"nuevo", "new"}, {"viejo", "old"},
	{"joven", "young"}, {"bueno", "good"}, {"malo", "bad"}, {"hermoso", "beautiful"},
	{"feo", "ugly"}, {"feliz", "happy"}, {"triste", "sad"}, {"enojado", "angry"},
	{"tranquilo", "calm"}, {"nervioso", "nervous"}, {"valiente", "brave"}, {"tímido", "shy"},
	{"inteligente", "intelligent"}, {"interesante", "interesting"}, {"aburrido", "boring"}, {"divertido", "fun"},
	{"fácil", "easy"}, {"difícil", "difficult"}, {"importante", "important"}, {"necesario", "necessary"},
	{"posible", "possible"}, {"imposible", "impossible"}, {"verdadero", "true"}, {"falso", "false"},
	{"limpio", "clean"}, {"sucio", "dirty"}, {"lleno", "full"}, {"vacío", "empty"},
	{"abierto", "open"}, {"cerrado", "closed"}, {"vivo", "alive"}, {"muerto", "dead"},
	{"rico", "rich"}, {"pobre", "poor"}, {"libre", "free"}, {"ocupado", "busy"},
}

var sentenceTemplates = []struct {
	ES string // placeholders: {N}, {N2}, {V}, {A}
	EN string
}{
	{"El {N} es {A}.", "The {N_EN} is {A_EN}."},
	{"La {N} está cerca del {N2}.", "The {N_EN} is near the {N2_EN}."},
	{"Yo quiero {V} con mi {N}.", "I want to {V_EN} with my {N_EN}."},
	{"¿Puedes {V} el {N}?", "Can you {V_EN} the {N_EN}?"},
	{"Mi {N} es muy {A}.", "My {N_EN} is very {A_EN}."},
	{"Vamos a {V} en el {N}.", "We are going to {V_EN} at the {N_EN}."},
	{"El {N} {A} está aquí.", "The {A_EN} {N_EN} is here."},
	{"Necesito {V} antes del {N}.", "I need to {V_EN} before the {N_EN}."},
	{"¿Dónde está tu {N}?", "Where is your {N_EN}?"},
	{"Ella va a {V} mañana.", "She is going to {V_EN} tomorrow."},
	{"Nosotros {V} todos los días.", "We {V_EN} every day."},
	{"Este {N} es más {A} que el otro.", "This {N_EN} is more {A_EN} than the other one."},
	{"El {N} de mi amigo es {A}.", "My friend's {N_EN} is {A_EN}."},
	{"Quiero un {N} {A}, por favor.", "I want a {A_EN} {N_EN}, please."},
	{"Hay un {N} en la {N2}.", "There is a {N_EN} on the {N2_EN}."},
	{"No puedo {V} sin el {N}.", "I cannot {V_EN} without the {N_EN}."},
	{"El {N} de la {N2} es {A}.", "The {N2_EN}'s {N_EN} is {A_EN}."},
	{"Ellos van a {V} la {N}.", "They are going to {V_EN} the {N_EN}."},
}

var grammarStems = []string{
	"In Spanish, the verb '%s' is regular and follows the standard %s conjugation pattern.",
	"Note that '%s' agrees in gender and number with the noun '%s' it modifies.",
	"The construction '%s + %s' is commonly used in formal writing but rarely in everyday conversation.",
	"Spanish speakers often pair '%s' with the preposition '%s' when expressing direction or motion.",
	"Unlike English, the subject pronoun is usually omitted before '%s' because the verb ending already indicates the subject.",
	"The word '%s' is a false cognate: although it looks like an English word, its meaning is closer to '%s'.",
	"Pay attention to the placement of '%s'; in Spanish it typically follows the noun '%s', not precedes it.",
	"In the present tense, '%s' takes the same form for first-person singular and third-person singular when used reflexively with '%s'.",
	"Native speakers from Latin America may pronounce '%s' differently than speakers from Spain, especially the consonant before '%s'.",
	"The diminutive form of '%s' is created by adding the suffix '-ito' or '-ita', producing a meaning closer to 'little %s' or 'dear %s'.",
}

var culturalStems = []string{
	"In many Spanish-speaking countries, '%s' is associated with the cultural tradition of gathering for a long midday '%s'.",
	"The festival of '%s', celebrated annually in several regions, traditionally involves '%s' shared among neighbors.",
	"Older generations often use '%s' as a term of endearment when speaking with a '%s', though younger speakers prefer alternatives.",
	"In rural areas, the '%s' is still hand-prepared, while in cities it is more common to buy a ready-made '%s' from the market.",
	"The phrase containing '%s' carries different connotations in Mexico versus Argentina; in one it implies '%s', in the other something closer to playfulness.",
	"During Sunday gatherings, families typically share a '%s' while discussing the events of the past '%s'.",
	"Street vendors in coastal cities often sell '%s' alongside fresh '%s', a combination unique to that region.",
	"The word '%s' can refer to either a physical '%s' or, metaphorically, to a state of mind, depending on context.",
}

var skillTags = []string{
	"present-tense", "preterite", "imperfect", "future", "conditional",
	"subjunctive", "regular-ar", "regular-er", "regular-ir", "irregular",
	"ser-vs-estar", "por-vs-para", "gender-agreement", "number-agreement",
	"reflexive", "object-pronouns", "comparatives", "superlatives",
	"direct-object", "indirect-object", "question-formation", "negation",
	"diminutives", "augmentatives", "false-cognates", "idiomatic",
}

var exerciseTypes = []string{
	"translation_es_to_en",
	"translation_en_to_es",
	"listening_transcribe",
	"fill_blank",
	"multiple_choice",
	"match_pairs",
	"conjugation",
	"sentence_order",
}
