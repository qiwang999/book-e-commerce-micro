package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/qiwang/book-e-commerce-micro/service/book/model"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var defaultSubjects = []string{
	// Literature & Fiction
	"fiction", "literary_fiction", "classic_literature", "short_stories",
	"historical_fiction", "adventure", "humor", "satire",
	// Genre Fiction
	"science_fiction", "fantasy", "mystery", "romance", "thriller",
	"horror", "crime", "detective", "suspense", "dystopian",
	"young_adult_fiction", "urban_fantasy", "cyberpunk", "steampunk",
	"paranormal", "western", "war_fiction", "spy_fiction",
	// Non-fiction
	"biography", "autobiography", "memoir", "true_crime",
	"journalism", "essays", "self_help", "personal_development",
	// History & Social Sciences
	"history", "world_history", "american_history", "european_history",
	"ancient_history", "medieval_history", "military_history",
	"politics", "political_science", "sociology", "anthropology",
	"archaeology", "geography", "cultural_studies",
	// Philosophy & Religion
	"philosophy", "ethics", "logic", "metaphysics",
	"religion", "theology", "spirituality", "mythology", "buddhism",
	// Psychology & Mind
	"psychology", "cognitive_science", "neuroscience", "psychiatry",
	"behavioral_science", "psychotherapy",
	// Science & Nature
	"science", "physics", "chemistry", "biology", "ecology",
	"astronomy", "geology", "genetics", "evolution",
	"environmental_science", "climate", "zoology", "botany",
	"marine_biology", "paleontology",
	// Technology & Computing
	"programming", "computer_science", "software_engineering",
	"artificial_intelligence", "machine_learning", "data_science",
	"web_development", "databases", "networking", "cybersecurity",
	"operating_systems", "algorithms", "robotics",
	// Mathematics
	"mathematics", "algebra", "statistics", "probability",
	"geometry", "calculus", "number_theory",
	// Business & Economics
	"business", "economics", "finance", "investing", "marketing",
	"management", "entrepreneurship", "accounting", "leadership",
	"real_estate", "international_trade",
	// Arts & Culture
	"art", "music", "film", "photography", "architecture",
	"design", "theater", "dance", "painting", "sculpture",
	// Language & Education
	"education", "teaching", "linguistics", "language",
	"writing", "grammar", "literacy", "poetry",
	// Health & Lifestyle
	"health", "medicine", "nutrition", "fitness", "cooking",
	"gardening", "sports", "yoga", "meditation",
	"diet", "mental_health", "public_health",
	// Children & Young Adult
	"children", "picture_books", "juvenile_fiction", "juvenile_nonfiction",
	"fairy_tales", "young_adult", "middle_grade",
	// Travel & Reference
	"travel", "maps", "reference", "encyclopedias", "dictionaries",
	// Law & Government
	"law", "constitutional_law", "criminal_law", "international_law",
	"government", "public_policy", "civil_rights",
	// Engineering & Applied Science
	"engineering", "electrical_engineering", "mechanical_engineering",
	"civil_engineering", "aerospace", "nanotechnology",
	// Other
	"graphic_novels", "comics", "manga", "games", "puzzles",
	"crafts", "pets", "animals", "nature", "environment",
	"transportation", "agriculture", "forestry", "oceanography",
}

var subjects100k = []string{
	"accessible_book", "lending_library", "in_library", "protected_daisy",
	"popular_print_disabled", "love", "friendship", "family",
	"death", "magic", "dragons", "vampires", "witches",
	"time_travel", "aliens", "space", "robots",
	"school", "college", "university", "teachers",
	"dogs", "cats", "horses", "birds", "insects",
	"ocean", "mountains", "forests", "rivers", "deserts",
	"china", "japan", "india", "africa", "europe",
	"france", "germany", "russia", "brazil", "australia",
	"new_york", "london", "paris", "tokyo",
	"world_war_ii", "civil_war", "cold_war", "vietnam_war",
	"industrial_revolution", "renaissance", "ancient_rome", "ancient_greece",
	"bible", "quran", "hinduism", "christianity", "islam",
	"jazz", "rock_music", "classical_music", "hip_hop", "folk_music",
	"opera", "symphony", "piano", "guitar", "violin",
	"soccer", "basketball", "baseball", "football", "tennis",
	"olympics", "swimming", "cycling", "martial_arts", "boxing",
	"wine", "beer", "coffee", "tea", "chocolate",
	"bread", "baking", "vegetarian", "vegan", "italian_cooking",
	"french_cooking", "asian_cooking", "mexican_cooking",
	"democracy", "communism", "capitalism", "socialism", "feminism",
	"civil_rights", "human_rights", "labor", "immigration",
	"climate_change", "renewable_energy", "nuclear_energy", "sustainability",
	"biotechnology", "pharmaceutical", "surgery", "anatomy", "physiology",
	"cancer", "diabetes", "heart_disease", "infectious_disease",
	"astronomy", "cosmology", "quantum_physics", "thermodynamics",
	"organic_chemistry", "biochemistry", "molecular_biology",
	"microeconomics", "macroeconomics", "game_theory", "behavioral_economics",
	"stock_market", "cryptocurrency", "banking", "insurance",
	"python", "java", "javascript", "c++", "rust",
	"linux", "docker", "kubernetes", "cloud_computing", "aws",
	"deep_learning", "natural_language_processing", "computer_vision",
	"blockchain", "internet_of_things", "virtual_reality", "augmented_reality",
}

type searchConfig struct {
	subjects   []string
	perSubject int
	total      int
	fillDesc   bool
}

type olSearchResponse struct {
	NumFound int            `json:"numFound"`
	Docs     []olSearchDoc  `json:"docs"`
}

type olSearchDoc struct {
	Key                string   `json:"key"`
	Title              string   `json:"title"`
	AuthorName         []string `json:"author_name"`
	ISBN               []string `json:"isbn"`
	Publisher          []string `json:"publisher"`
	FirstPublishYear   int      `json:"first_publish_year"`
	Subject            []string `json:"subject"`
	CoverI             int      `json:"cover_i"`
	NumberOfPagesMedian int     `json:"number_of_pages_median"`
	Language           []string `json:"language"`
	RatingsAverage     float64  `json:"ratings_average"`
	RatingsCount       int64    `json:"ratings_count"`
}

func runSearchMode(ctx context.Context, hc *http.Client, rl *rateLimiter, coll *mongo.Collection, cfg searchConfig) {
	seen := make(map[string]struct{})

	existing, _ := loadExistingISBNs(ctx, coll)
	for _, isbn := range existing {
		seen[isbn] = struct{}{}
	}
	log.Printf("[search] %d existing ISBNs loaded for dedup", len(seen))

	allSubjects := cfg.subjects
	if cfg.total >= 50000 {
		extra := make(map[string]struct{})
		for _, s := range allSubjects {
			extra[s] = struct{}{}
		}
		for _, s := range subjects100k {
			if _, ok := extra[s]; !ok {
				allSubjects = append(allSubjects, s)
				extra[s] = struct{}{}
			}
		}
		log.Printf("[search] 100k mode: using %d subjects total", len(allSubjects))
	}

	totalInserted := 0
	totalSkipped := 0

	for _, subject := range allSubjects {
		if totalInserted >= cfg.total {
			break
		}
		subjectInserted := 0
		offset := 0
		pageSize := 100

		for subjectInserted < cfg.perSubject && totalInserted < cfg.total {
			u := fmt.Sprintf("%s/search.json?subject=%s&limit=%d&offset=%d&fields=key,title,author_name,isbn,publisher,first_publish_year,subject,cover_i,number_of_pages_median,language,ratings_average,ratings_count",
				olBase, url.QueryEscape(subject), pageSize, offset)

			var resp olSearchResponse
			if err := getJSON(ctx, hc, rl, u, &resp); err != nil {
				log.Printf("[search] subject=%s offset=%d error: %v", subject, offset, err)
				break
			}

			if len(resp.Docs) == 0 {
				break
			}

			batch := make([]mongo.WriteModel, 0, len(resp.Docs))
			for i := range resp.Docs {
				doc := &resp.Docs[i]
				isbn := pickISBN(doc.ISBN)
				if isbn == "" {
					totalSkipped++
					continue
				}
				if _, ok := seen[isbn]; ok {
					totalSkipped++
					continue
				}
				seen[isbn] = struct{}{}

				b := searchDocToBook(doc, isbn, subject)
				filter := bson.M{"isbn": isbn}
				update := bson.M{"$setOnInsert": b}
				wm := mongo.NewUpdateOneModel().SetFilter(filter).SetUpdate(update).SetUpsert(true)
				batch = append(batch, wm)
			}

			if len(batch) > 0 {
				res, err := coll.BulkWrite(ctx, batch, options.BulkWrite().SetOrdered(false))
				if err != nil {
					log.Printf("[search] bulk write error subject=%s: %v", subject, err)
				}
				inserted := int(res.UpsertedCount)
				subjectInserted += inserted
				totalInserted += inserted
			}

			offset += pageSize
			if offset >= resp.NumFound || len(resp.Docs) < pageSize {
				break
			}
		}
		log.Printf("[search] subject=%-20s inserted=%d (total=%d)", subject, subjectInserted, totalInserted)
	}

	log.Printf("[search] done: inserted=%d skipped=%d", totalInserted, totalSkipped)

	if cfg.fillDesc && totalInserted > 0 {
		backfillDescriptions(ctx, hc, rl, coll)
	}
}

func pickISBN(isbns []string) string {
	var best string
	for _, s := range isbns {
		s = strings.TrimSpace(s)
		if len(s) == 13 {
			return s
		}
		if len(s) == 10 && best == "" {
			best = s
		}
	}
	return best
}

func searchDocToBook(doc *olSearchDoc, isbn, querySubject string) model.Book {
	author := "Unknown"
	if len(doc.AuthorName) > 0 {
		author = strings.Join(doc.AuthorName, " / ")
	}

	publisher := ""
	if len(doc.Publisher) > 0 {
		publisher = doc.Publisher[0]
	}

	publishDate := ""
	if doc.FirstPublishYear > 0 {
		publishDate = fmt.Sprintf("%d", doc.FirstPublishYear)
	}

	lang := "en"
	if len(doc.Language) > 0 {
		lang = mapSearchLanguage(doc.Language[0])
	}

	category, subcategory, tags := subjectsToCategories(doc.Subject)
	if category == "General" || category == "" {
		category = titleCase(strings.ReplaceAll(querySubject, "_", " "))
	}

	coverURL := ""
	if doc.CoverI > 0 {
		coverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", doc.CoverI)
	} else {
		coverURL = fmt.Sprintf("https://covers.openlibrary.org/b/isbn/%s-L.jpg", isbn)
	}

	now := time.Now()
	return model.Book{
		Title:       strings.TrimSpace(doc.Title),
		Author:      author,
		ISBN:        isbn,
		Publisher:   publisher,
		PublishDate: publishDate,
		Price:       0,
		Category:    category,
		Subcategory: subcategory,
		Description: "",
		CoverURL:    coverURL,
		Pages:       int32(doc.NumberOfPagesMedian),
		Language:    lang,
		Tags:        tags,
		Rating:      doc.RatingsAverage,
		RatingCount: doc.RatingsCount,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func mapSearchLanguage(lang string) string {
	lang = strings.ToLower(strings.TrimSpace(lang))
	switch lang {
	case "eng":
		return "en"
	case "chi", "zho":
		return "zh-CN"
	case "jpn":
		return "ja"
	case "fre", "fra":
		return "fr"
	case "spa":
		return "es"
	case "ger", "deu":
		return "de"
	default:
		return lang
	}
}

func loadExistingISBNs(ctx context.Context, coll *mongo.Collection) ([]string, error) {
	cur, err := coll.Find(ctx, bson.M{"isbn": bson.M{"$ne": ""}},
		options.Find().SetProjection(bson.M{"isbn": 1}).SetBatchSize(5000))
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var isbns []string
	for cur.Next(ctx) {
		isbn, _ := cur.Current.Lookup("isbn").StringValueOK()
		if isbn != "" {
			isbns = append(isbns, isbn)
		}
	}
	return isbns, nil
}

func backfillDescriptions(ctx context.Context, hc *http.Client, rl *rateLimiter, coll *mongo.Collection) {
	log.Println("[fill-desc] starting description backfill...")

	filter := bson.M{
		"$or": []bson.M{
			{"description": ""},
			{"description": bson.M{"$exists": false}},
		},
	}
	cur, err := coll.Find(ctx, filter, options.Find().SetProjection(bson.M{"isbn": 1}).SetBatchSize(500))
	if err != nil {
		log.Printf("[fill-desc] find error: %v", err)
		return
	}
	defer cur.Close(ctx)

	type bookRef struct {
		ISBN string `bson:"isbn"`
	}
	var refs []bookRef
	if err := cur.All(ctx, &refs); err != nil {
		log.Printf("[fill-desc] cursor error: %v", err)
		return
	}
	log.Printf("[fill-desc] %d books need description", len(refs))

	filled := 0
	errors := 0
	for i, ref := range refs {
		if ref.ISBN == "" {
			continue
		}

		u := fmt.Sprintf("%s/search.json?isbn=%s&fields=key&limit=1", olBase, ref.ISBN)
		var sr olSearchResponse
		if err := getJSON(ctx, hc, rl, u, &sr); err != nil || len(sr.Docs) == 0 || sr.Docs[0].Key == "" {
			errors++
			continue
		}

		workKey := sr.Docs[0].Key
		var w workDoc
		if err := getJSON(ctx, hc, rl, olBase+workKey+".json", &w); err != nil {
			errors++
			continue
		}

		desc := normalizeDescription(w.Description)
		if desc == "" {
			continue
		}

		_, err := coll.UpdateOne(ctx, bson.M{"isbn": ref.ISBN}, bson.M{"$set": bson.M{"description": desc, "updated_at": time.Now()}})
		if err != nil {
			errors++
			continue
		}
		filled++

		if (i+1)%100 == 0 {
			log.Printf("[fill-desc] progress: %d/%d filled=%d errors=%d", i+1, len(refs), filled, errors)
		}
	}
	log.Printf("[fill-desc] done: filled=%d errors=%d total=%d", filled, errors, len(refs))
}
