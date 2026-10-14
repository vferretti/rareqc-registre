package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"registre-admin/internal/config"
	"registre-admin/internal/database"
	"registre-admin/internal/guid"
	"registre-admin/internal/types"

	"gorm.io/gorm"
)

// Québécois and anglophone first names
var boysNames = []string{
	"Olivier", "Liam", "Noah", "William", "Félix", "Thomas", "Léo", "Nathan",
	"Samuel", "Jacob", "Raphaël", "Émile", "Antoine", "Alexandre", "Mathis",
	"Étienne", "Xavier", "Gabriel", "Benjamin", "Alexis", "Édouard", "Louis",
	"Zachary", "Charles", "Arnaud", "Théo", "Julien", "Maxime", "Adam", "Simon",
	"Laurent", "Philippe", "Vincent", "Elliot", "Loïc", "Tristan", "Cédric",
	"Jérémy", "Hubert", "Damien",
	// Anglophone names
	"James", "Ryan", "Connor", "Dylan", "Tyler", "Brandon", "Kyle",
	"Matthew", "Andrew", "Kevin",
}

var girlsNames = []string{
	"Emma", "Alice", "Léa", "Florence", "Olivia", "Charlotte", "Rosalie",
	"Béatrice", "Camille", "Zoé", "Juliette", "Chloé", "Mia", "Laurence",
	"Élodie", "Élizabeth", "Simone", "Mégane", "Ariane", "Aurélie",
	"Gabrielle", "Amélie", "Jade", "Léonie", "Coralie", "Noémie", "Sofia",
	"Maélie", "Flavie", "Clara", "Annabelle", "Sarah", "Raphaëlle", "Ève",
	"Delphine", "Frédérique", "Madeleine", "Évelyne", "Justine", "Sandrine",
	// Anglophone names
	"Ashley", "Brittany", "Megan", "Heather", "Jessica", "Kaitlyn", "Lindsay",
	"Samantha", "Stephanie", "Rachel",
}

var lastNames = []string{
	"Tremblay", "Gagnon", "Roy", "Côté", "Bouchard", "Gauthier", "Morin",
	"Lavoie", "Fortin", "Gagné", "Ouellet", "Pelletier", "Bélanger", "Lévesque",
	"Bergeron", "Leblanc", "Paquette", "Girard", "Simard", "Boucher",
	"Caron", "Beaulieu", "Cloutier", "Dubé", "Poirier", "Fournier", "Lapointe",
	"Leclerc", "Lefebvre", "Martel", "Thibault", "Mercier", "Dupuis", "Hébert",
	"Villeneuve", "Desjardins", "Therrien", "Savard", "Nadeau", "Dufour",
	"Lalonde", "Proulx", "Breton", "Ménard", "Beaudoin", "Picard", "Langlois",
	"Brassard", "Tardif", "Champagne",
	// Anglophone last names
	"Smith", "Johnson", "Brown", "Wilson", "Taylor", "Campbell", "Stewart",
	"Murray", "Robertson", "Henderson",
}

var cities = []string{
	"Montréal", "Québec", "Laval", "Gatineau", "Longueuil", "Sherbrooke",
	"Saguenay", "Lévis", "Trois-Rivières", "Terrebonne", "Saint-Jean-sur-Richelieu",
	"Repentigny", "Drummondville", "Granby", "Saint-Hyacinthe", "Rimouski",
	"Victoriaville", "Shawinigan", "Rouyn-Noranda", "Val-d'Or",
}

var streets = []string{
	"rue Principale", "boulevard Laurier", "avenue du Parc", "rue Saint-Jean",
	"rue de la Montagne", "boulevard René-Lévesque", "rue Sainte-Catherine",
	"chemin du Roy", "avenue Cartier", "rue des Érables", "boulevard Charest",
	"rue Saint-Joseph", "avenue Maguire", "rue du Pont", "boulevard Hamel",
}

var authors = []string{
	"John Smith", "Marie Tremblay", "Pierre Gagnon", "Sophie Lavoie",
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("invalid configuration: %v", err)
	}
	// The seed wipes every table: never against a production database
	// unless explicitly forced.
	if cfg.Production && os.Getenv("SEED_ALLOW_DESTRUCTIVE") != "yes" {
		log.Fatalf("refusing to seed in production (GIN_MODE=release) on %s/%s: set SEED_ALLOW_DESTRUCTIVE=yes to force",
			cfg.DB.Host, cfg.DB.Name)
	}
	log.Printf("Seeding %s/%s (all existing data will be deleted)", cfg.DB.Host, cfg.DB.Name)
	db, err := database.NewPostgresDB(cfg.DB)
	if err != nil {
		log.Fatalf("failed to connect to postgres: %v", err)
	}

	// Clean existing data (order matters for FK)
	db.Exec("DELETE FROM communication")
	db.Exec("DELETE FROM cart_item")
	db.Exec("DELETE FROM external_id")
	db.Exec("DELETE FROM consent")
	db.Exec("DELETE FROM consent_clause")
	db.Exec("DELETE FROM document_file")
	db.Exec("DELETE FROM document")
	db.Exec("DELETE FROM activity_log")
	db.Exec("DELETE FROM guid")
	db.Exec("DELETE FROM contact")
	db.Exec("DELETE FROM participant")

	log.Println("Seeding 200 participants...")

	// Distribute participants across 7 quarters (Q3 2024 → Q1 2026)
	// with progressive growth: fewer early, more recent
	type quarterSlot struct {
		start time.Time
		end   time.Time
	}
	quarters := []quarterSlot{
		{time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 9, 30, 0, 0, 0, 0, time.UTC)},   // 24Q3
		{time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)}, // 24Q4
		{time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 3, 31, 0, 0, 0, 0, time.UTC)},   // 25Q1
		{time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)},   // 25Q2
		{time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 9, 30, 0, 0, 0, 0, time.UTC)},   // 25Q3
		{time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC), time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)}, // 25Q4
		{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)},   // 26Q1
	}
	// Progressive distribution: 10, 15, 20, 25, 35, 45, 50 = 200
	perQuarter := []int{10, 15, 20, 25, 35, 45, 50}

	// 85% children, 15% adults
	childCount := 170
	adultCount := 30
	totalCount := childCount + adultCount

	// Build a list of enrollment timestamps spread across quarters
	enrollTimes := make([]time.Time, 0, totalCount)
	for qi, count := range perQuarter {
		q := quarters[qi]
		dayRange := int(q.end.Sub(q.start).Hours() / 24)
		if dayRange < 1 {
			dayRange = 1
		}
		for j := 0; j < count; j++ {
			randomDay := rand.Intn(dayRange)
			randomHour := rand.Intn(10) + 8 // 08:00-17:59
			ts := q.start.Add(time.Duration(randomDay)*24*time.Hour + time.Duration(randomHour)*time.Hour)
			enrollTimes = append(enrollTimes, ts)
		}
	}
	// Shuffle to mix children and adults
	rand.Shuffle(len(enrollTimes), func(i, j int) {
		enrollTimes[i], enrollTimes[j] = enrollTimes[j], enrollTimes[i]
	})

	for i := 0; i < childCount; i++ {
		ts := enrollTimes[i]
		seedChild(db, i, ts)
	}
	for i := 0; i < adultCount; i++ {
		ts := enrollTimes[childCount+i]
		seedAdult(db, childCount+i, ts)
	}

	// Override created_at to match enrollment timestamps (GORM autoCreateTime sets to NOW)
	var allParticipants []types.Participant
	db.Order("id ASC").Find(&allParticipants)
	for i, p := range allParticipants {
		if i < len(enrollTimes) {
			db.Exec("UPDATE participant SET created_at = ? WHERE id = ?", enrollTimes[i], p.ID)
		}
	}

	// Seed consent templates and clauses (demo/dev data)
	seedConsentTemplates(db)

	// Load consent PDF into document table
	seedConsentDocuments(db)

	// Seed consents
	seedConsents(db)

	// Seed external systems and IDs
	seedExternalSystems(db)

	log.Printf("Seed complete: %d participants created across 7 quarters with activity logs, consents, and external IDs", totalCount)
}

func createActivityLog(db *gorm.DB, actionTypeCode string, participantID int, author string, details string, createdAt time.Time) {
	var detailsPtr *string
	if details != "" {
		detailsPtr = &details
	}
	db.Create(&types.ActivityLog{
		ActionTypeCode: actionTypeCode,
		ParticipantID:  &participantID,
		Author:         author,
		Details:        detailsPtr,
		CreatedAt:      createdAt,
	})
}

func seedChild(db *gorm.DB, index int, ts time.Time) {
	isFemale := index%2 == 0
	sex := "male"
	firstName := pick(boysNames)
	if isFemale {
		sex = "female"
		firstName = pick(girlsNames)
	}

	lastName := pick(lastNames)
	dob := randomChildDOB()
	city := pick(cities)
	lang := langForName(firstName)
	address := randomAddress()
	contactCity := pick(cities)
	postalCode := randomPostalCode()
	author := pick(authors)

	ramq := generateRAMQ(firstName, lastName, dob, isFemale)
	participant := types.Participant{
		FirstName:       firstName,
		LastName:        lastName,
		DateOfBirth:     dob,
		CityOfBirth:     &city,
		RAMQ:            &ramq,
		SexAtBirthCode:  sex,
		VitalStatusCode: "alive",
		CreatedAt:       ts,
	}
	db.Create(&participant)
	db.Create(guid.Compute(&participant))

	// Activity: participant created
	createActivityLog(db, "participant_created", participant.ID, author,
		fmt.Sprintf("%s %s", firstName, lastName), ts)

	// Self contact (participant's own coordinates)
	selfContact := types.Contact{
		ParticipantID:     participant.ID,
		FirstName:         firstName,
		LastName:          lastName,
		RelationshipCode:  "self",
		IsPrimary:         false,
		Email:             fmt.Sprintf("%s.%s@%s", lower(firstName), lower(lastName), randomDomain()),
		Phone:             randomPhone(),
		StreetAddress:     address,
		City:              contactCity,
		Province:          "QC",
		CodePostal:        postalCode,
		PreferredLanguage: lang,
	}
	db.Create(&selfContact)

	// Mother contact (always present for children, is primary)
	motherFirst := pick(girlsNames)
	motherLast := lastName
	if rand.Intn(5) == 0 {
		motherLast = pick(lastNames) // ~20% different last name
	}

	motherContact := types.Contact{
		ParticipantID:     participant.ID,
		FirstName:         motherFirst,
		LastName:          motherLast,
		RelationshipCode:  "mother",
		IsPrimary:         true,
		Email:             fmt.Sprintf("%s.%s@%s", lower(motherFirst), lower(motherLast), randomDomain()),
		Phone:             randomPhone(),
		StreetAddress:     address,
		City:              contactCity,
		Province:          "QC",
		CodePostal:        postalCode,
		PreferredLanguage: lang,
	}
	db.Create(&motherContact)

	// ~30% chance participant was edited later
	if rand.Intn(100) < 30 {
		editTs := ts.Add(time.Duration(rand.Intn(48)+1) * time.Hour)
		editAuthor := pick(authors)
		createActivityLog(db, "participant_edited", participant.ID, editAuthor,
			fmt.Sprintf("%s %s", firstName, lastName), editTs)
	}

	// Father contact (~40% of children)
	if rand.Intn(5) < 2 {
		fatherFirst := pick(boysNames)
		fatherLast := lastName

		fatherContact := types.Contact{
			ParticipantID:     participant.ID,
			FirstName:         fatherFirst,
			LastName:          fatherLast,
			RelationshipCode:  "father",
			IsPrimary:         false,
			Email:             fmt.Sprintf("%s.%s@%s", lower(fatherFirst), lower(fatherLast), randomDomain()),
			Phone:             randomPhone(),
			StreetAddress:     address,
			City:              contactCity,
			Province:          "QC",
			CodePostal:        postalCode,
			PreferredLanguage: lang,
		}
		db.Create(&fatherContact)
	}
}

func seedAdult(db *gorm.DB, index int, ts time.Time) {
	isFemale := index%2 == 0
	sex := "male"
	firstName := pick(boysNames)
	if isFemale {
		sex = "female"
		firstName = pick(girlsNames)
	}

	lastName := pick(lastNames)
	dob := randomAdultDOB()
	city := pick(cities)
	author := pick(authors)

	ramq := generateRAMQ(firstName, lastName, dob, isFemale)
	participant := types.Participant{
		FirstName:       firstName,
		LastName:        lastName,
		DateOfBirth:     dob,
		CityOfBirth:     &city,
		RAMQ:            &ramq,
		SexAtBirthCode:  sex,
		VitalStatusCode: "alive",
		CreatedAt:       ts,
	}
	db.Create(&participant)
	db.Create(guid.Compute(&participant))

	// Activity: participant created
	createActivityLog(db, "participant_created", participant.ID, author,
		fmt.Sprintf("%s %s", firstName, lastName), ts)

	// Adult is their own primary contact
	lang := langForName(firstName)
	selfContact := types.Contact{
		ParticipantID:     participant.ID,
		FirstName:         firstName,
		LastName:          lastName,
		RelationshipCode:  "self",
		IsPrimary:         true,
		Email:             fmt.Sprintf("%s.%s@%s", lower(firstName), lower(lastName), randomDomain()),
		Phone:             randomPhone(),
		StreetAddress:     randomAddress(),
		City:              pick(cities),
		Province:          "QC",
		CodePostal:        randomPostalCode(),
		PreferredLanguage: lang,
	}
	db.Create(&selfContact)

	// ~25% chance participant was edited later
	if rand.Intn(100) < 25 {
		editTs := ts.Add(time.Duration(rand.Intn(96)+1) * time.Hour)
		editAuthor := pick(authors)
		createActivityLog(db, "participant_edited", participant.ID, editAuthor,
			fmt.Sprintf("%s %s", firstName, lastName), editTs)
	}
}

// seedConsentTemplates creates the consent template documents and their clauses (demo/dev data).
func seedConsentTemplates(db *gorm.DB) {
	// Template 1: RareQc — 2 clauses (registry + recontact)
	rareqcDoc := types.Document{
		Name:     "Formulaire de consentement – RareQc",
		FileName: "Consentement_RareQc.pdf",
		TypeCode: "consent_template",
		MimeType: "application/pdf",
	}
	if err := db.Where("file_name = ?", rareqcDoc.FileName).FirstOrCreate(&rareqcDoc).Error; err != nil {
		log.Printf("Error creating RareQc template: %v", err)
	}

	rareqcClauses := []types.ConsentClause{
		{TemplateDocumentID: rareqcDoc.ID, ClauseTypeCode: "registry",
			ClauseFr: "Je consens à faire partie du registre RareQc.",
			ClauseEn: "I consent to be part of the RareQc registry."},
		{TemplateDocumentID: rareqcDoc.ID, ClauseTypeCode: "recontact",
			ClauseFr: "Je consens à être recontacté(e) pour des recherches futures ou des informations liées au registre RareQc.",
			ClauseEn: "I consent to be recontacted for future research or information related to the RareQc registry."},
	}
	for _, clause := range rareqcClauses {
		db.Where("clause_type_code = ? AND template_document_id = ?", clause.ClauseTypeCode, rareqcDoc.ID).FirstOrCreate(&clause)
	}

	// Template 2: RQDM — 3 clauses (registry + recontact + external linkage)
	rqdmDoc := types.Document{
		Name:     "Formulaire de consentement – RQDM",
		FileName: "Consentement_RQDM.pdf",
		TypeCode: "consent_template",
		MimeType: "application/pdf",
	}
	if err := db.Where("name = ?", rqdmDoc.Name).FirstOrCreate(&rqdmDoc).Error; err != nil {
		log.Printf("Error creating RQDM template: %v", err)
	}

	rqdmClauses := []types.ConsentClause{
		{TemplateDocumentID: rqdmDoc.ID, ClauseTypeCode: "registry",
			ClauseFr: "Je consens à faire partie du registre RareQc et à ce que mes échantillons biologiques, mes données cliniques et génomiques soient conservés et utilisés à des fins de diagnostic moléculaire et de recherche en génomique.",
			ClauseEn: "I consent to be part of the RareQc registry and to having my biological samples, clinical and genomic data stored and used for molecular diagnostic and genomic research purposes."},
		{TemplateDocumentID: rqdmDoc.ID, ClauseTypeCode: "recontact",
			ClauseFr: "Je consens à être recontacté(e) si de nouvelles découvertes pertinentes à ma condition de santé sont identifiées, ou pour être invité(e) à participer à de nouvelles études de recherche.",
			ClauseEn: "I consent to being recontacted if new findings relevant to my health condition are identified, or to be invited to participate in new research studies."},
		{TemplateDocumentID: rqdmDoc.ID, ClauseTypeCode: "external_linkage",
			ClauseFr: "Je consens à ce que mes données soient liées à des bases de données provinciales, nationales ou internationales (ex. ClinVar, OMIM, bases hospitalières) à des fins de recherche et d'amélioration du diagnostic.",
			ClauseEn: "I consent to having my data linked to provincial, national, or international databases (e.g., ClinVar, OMIM, hospital databases) for research and diagnostic improvement purposes."},
	}
	for _, clause := range rqdmClauses {
		db.Where("clause_type_code = ? AND template_document_id = ?", clause.ClauseTypeCode, rqdmDoc.ID).FirstOrCreate(&clause)
	}

	log.Println("Consent templates and clauses seeded")
}

// loadTemplatePDF loads a PDF file into a template document's document_file row.
func loadTemplatePDF(db *gorm.DB, fileName, pdfPath string) {
	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		log.Printf("Warning: could not read %s: %v", pdfPath, err)
		return
	}
	var doc types.Document
	if err := db.Where("file_name = ?", fileName).First(&doc).Error; err != nil {
		log.Printf("Warning: document %q not found in DB: %v", fileName, err)
		return
	}
	doc.FileSize = int64(len(pdfData))
	db.Save(&doc)
	file := types.DocumentFile{DocumentID: doc.ID, Data: pdfData}
	db.Where("document_id = ?", doc.ID).FirstOrCreate(&file)
	log.Printf("Template PDF loaded: %s (%d bytes)", doc.Name, len(pdfData))
}

// pdfDir resolves the directory holding the consent PDFs: SEED_PDF_DIR if
// set, /data inside the seed docker image, scripts/seed when run from
// backend/ with `go run ./scripts/seed/`.
func pdfDir() string {
	if d := os.Getenv("SEED_PDF_DIR"); d != "" {
		return d
	}
	if _, err := os.Stat("/data/Consentement_RareQc.pdf"); err == nil {
		return "/data"
	}
	return "scripts/seed"
}

// seedConsentDocuments loads both template PDFs into the database.
func seedConsentDocuments(db *gorm.DB) {
	dir := pdfDir()
	loadTemplatePDF(db, "Consentement_RareQc.pdf", filepath.Join(dir, "Consentement_RareQc.pdf"))
	loadTemplatePDF(db, "Consentement_RQDM.pdf", filepath.Join(dir, "Consentement_RQDM.pdf"))
}

// seedConsents creates consent records for all participants.
// Each participant gets one template (70% RareQc, 30% RQDM).
// Adults sign for themselves, minors are signed by their primary contact.
// A signed document is created per participant.
func seedConsents(db *gorm.DB) {
	// Fetch templates with their clauses
	var rareqcDoc, rqdmDoc types.Document
	db.Where("name LIKE ?", "%RareQc%").First(&rareqcDoc)
	db.Where("name LIKE ?", "%RQDM%").First(&rqdmDoc)

	var rareqcClauses, rqdmClauses []types.ConsentClause
	db.Where("template_document_id = ?", rareqcDoc.ID).Find(&rareqcClauses)
	db.Where("template_document_id = ?", rqdmDoc.ID).Find(&rqdmClauses)

	if len(rareqcClauses) == 0 && len(rqdmClauses) == 0 {
		log.Println("No consent clauses found, skipping consent seeding")
		return
	}

	// Load template PDFs for copying into signed documents
	dir := pdfDir()
	rareqcPDF, _ := os.ReadFile(filepath.Join(dir, "Consentement_RareQc.pdf"))
	rqdmPDF, _ := os.ReadFile(filepath.Join(dir, "Consentement_RQDM.pdf"))

	var participants []types.Participant
	db.Preload("Contacts").Find(&participants)

	now := time.Now()
	for _, p := range participants {
		// Determine signer
		age := now.Year() - p.DateOfBirth.Year()
		if now.YearDay() < p.DateOfBirth.YearDay() {
			age--
		}
		isMinor := age < 18

		var signerID *int
		if isMinor {
			for _, ct := range p.Contacts {
				if ct.IsPrimary && ct.RelationshipCode != "self" {
					id := ct.ID
					signerID = &id
					break
				}
			}
		} else {
			// Adult signs for themselves (self contact)
			for _, ct := range p.Contacts {
				if ct.RelationshipCode == "self" {
					id := ct.ID
					signerID = &id
					break
				}
			}
		}

		// Pick a template: 70% RareQc, 30% RQDM
		clauses := rareqcClauses
		templateName := "RareQc"
		pdfData := rareqcPDF
		if rand.Intn(100) < 30 {
			clauses = rqdmClauses
			templateName = "RQDM"
			pdfData = rqdmPDF
		}

		// Create a signed document for this participant with the template PDF as content
		fileName := fmt.Sprintf("Consentement_%s_%s_%s.pdf", p.FirstName, p.LastName, templateName)
		signedDoc := types.Document{
			Name:     fmt.Sprintf("Consentement signé — %s %s (%s)", p.FirstName, p.LastName, templateName),
			FileName: fileName,
			TypeCode: "consent_signed",
			MimeType: "application/pdf",
			FileSize: int64(len(pdfData)),
		}
		db.Create(&signedDoc)
		if len(pdfData) > 0 {
			db.Create(&types.DocumentFile{DocumentID: signedDoc.ID, Data: pdfData})
		}

		// Consent date: random within 7 days of participant creation
		consentDate := p.CreatedAt.Add(-time.Duration(rand.Intn(7)) * 24 * time.Hour)
		consentDate = time.Date(consentDate.Year(), consentDate.Month(), consentDate.Day(), 0, 0, 0, 0, time.UTC)
		author := pick(authors)
		docID := signedDoc.ID

		// Phase 1: create all consents as valid
		type consentRecord struct {
			consent    types.Consent
			clauseType string
		}
		var created []consentRecord
		for i, clause := range clauses {
			skipRate := []int{5, 15, 30}
			if i < len(skipRate) && rand.Intn(100) < skipRate[i] {
				continue
			}

			consent := types.Consent{
				ClauseID:      clause.ID,
				ParticipantID: p.ID,
				Date:          consentDate,
				StatusCode:    "valid",
				SignedByID:    signerID,
				DocumentID:    &docID,
			}
			db.Create(&consent)
			created = append(created, consentRecord{consent: consent, clauseType: clause.ClauseTypeCode})

			addedAt := consentDate.Add(time.Duration(rand.Intn(8)) * time.Hour)
			details := fmt.Sprintf("%s — valid", clause.ClauseTypeCode)
			createActivityLog(db, "consent_added", p.ID, author, details, addedAt)
		}

		// Phase 2: randomly expire/withdraw some consents
		// Business rule: if registry is withdrawn or expired, all other clauses get the same status
		var registryCascadeStatus string
		var cascadeDate time.Time
		var cascadeAuthor string

		for i := range created {
			if created[i].clauseType != "registry" {
				continue
			}
			roll := rand.Intn(100)
			if roll < 25 {
				newStatus := "expired"
				if roll >= 15 {
					newStatus = "withdrawn"
				}
				cascadeDate = consentDate.Add(time.Duration(rand.Intn(18)+3) * 24 * time.Hour)
				cascadeDate = time.Date(cascadeDate.Year(), cascadeDate.Month(), cascadeDate.Day(), 0, 0, 0, 0, time.UTC)
				cascadeAuthor = pick(authors)

				created[i].consent.StatusCode = newStatus
				created[i].consent.Date = cascadeDate
				db.Save(&created[i].consent)

				editDetails := fmt.Sprintf("%s — valid → %s", created[i].clauseType, newStatus)
				createActivityLog(db, "consent_edited", p.ID, cascadeAuthor, editDetails, cascadeDate.Add(time.Duration(rand.Intn(8))*time.Hour))

				registryCascadeStatus = newStatus
			}
		}

		for i := range created {
			if created[i].clauseType == "registry" {
				continue
			}
			if registryCascadeStatus != "" {
				// Cascade: registry withdrawn/expired → all other clauses get same status
				created[i].consent.StatusCode = registryCascadeStatus
				created[i].consent.Date = cascadeDate
				db.Save(&created[i].consent)

				editDetails := fmt.Sprintf("%s — valid → %s (registre %s)", created[i].clauseType, registryCascadeStatus, registryCascadeStatus)
				createActivityLog(db, "consent_edited", p.ID, cascadeAuthor, editDetails, cascadeDate.Add(time.Duration(rand.Intn(8))*time.Hour))
			} else {
				// Independent expire/withdraw for non-registry clauses
				roll := rand.Intn(100)
				if roll < 20 {
					newStatus := "expired"
					if roll >= 12 {
						newStatus = "withdrawn"
					}
					editDate := consentDate.Add(time.Duration(rand.Intn(18)+3) * 24 * time.Hour)
					editDate = time.Date(editDate.Year(), editDate.Month(), editDate.Day(), 0, 0, 0, 0, time.UTC)

					created[i].consent.StatusCode = newStatus
					created[i].consent.Date = editDate
					db.Save(&created[i].consent)

					editDetails := fmt.Sprintf("%s — valid → %s", created[i].clauseType, newStatus)
					createActivityLog(db, "consent_edited", p.ID, pick(authors), editDetails, editDate.Add(time.Duration(rand.Intn(8))*time.Hour))
				}
			}
		}
	}

	log.Printf("Consents seeded for %d participants", len(participants))
}

// --- Helpers ---

func pick(list []string) string {
	return list[rand.Intn(len(list))]
}

func lower(s string) string {
	// Simple ASCII-safe lowercase for email generation
	result := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			result = append(result, byte(r+32))
		case r == 'é' || r == 'è' || r == 'ê' || r == 'ë' || r == 'É':
			result = append(result, 'e')
		case r == 'à' || r == 'â' || r == 'ä':
			result = append(result, 'a')
		case r == 'î' || r == 'ï':
			result = append(result, 'i')
		case r == 'ô' || r == 'ö':
			result = append(result, 'o')
		case r == 'ù' || r == 'û' || r == 'ü':
			result = append(result, 'u')
		case r == 'ç':
			result = append(result, 'c')
		case r == 'ë':
			result = append(result, 'e')
		default:
			if r >= 'a' && r <= 'z' {
				result = append(result, byte(r))
			}
		}
	}
	return string(result)
}

func randomChildDOB() time.Time {
	// Children: 0-17 years old
	now := time.Now()
	yearsAgo := rand.Intn(17) + 1
	dob := now.AddDate(-yearsAgo, -rand.Intn(12), -rand.Intn(28))
	return time.Date(dob.Year(), dob.Month(), dob.Day(), 0, 0, 0, 0, time.UTC)
}

func randomAdultDOB() time.Time {
	// Adults: 18-65 years old
	now := time.Now()
	yearsAgo := rand.Intn(47) + 18
	dob := now.AddDate(-yearsAgo, -rand.Intn(12), -rand.Intn(28))
	return time.Date(dob.Year(), dob.Month(), dob.Day(), 0, 0, 0, 0, time.UTC)
}

func randomPhone() string {
	// Quebec area codes
	areaCodes := []string{"514", "438", "450", "418", "581", "819", "873", "367"}
	return fmt.Sprintf("%s%03d%04d", pick(areaCodes), rand.Intn(1000), rand.Intn(10000))
}

func randomAddress() string {
	return fmt.Sprintf("%d %s", rand.Intn(9999)+1, pick(streets))
}

func randomPostalCode() string {
	letters := "ABCEGHJKLMNPRSTVXY"
	return fmt.Sprintf("%c%d%c %d%c%d",
		letters[rand.Intn(len(letters))],
		rand.Intn(10),
		letters[rand.Intn(len(letters))],
		rand.Intn(10),
		letters[rand.Intn(len(letters))],
		rand.Intn(10),
	)
}

var angloFirstNames = map[string]bool{
	"James": true, "Ryan": true, "Connor": true, "Dylan": true, "Tyler": true,
	"Brandon": true, "Kyle": true, "Matthew": true, "Andrew": true, "Kevin": true,
	"Ashley": true, "Brittany": true, "Megan": true, "Heather": true, "Jessica": true,
	"Kaitlyn": true, "Lindsay": true, "Samantha": true, "Stephanie": true, "Rachel": true,
}

func langForName(firstName string) string {
	if angloFirstNames[firstName] {
		return "en"
	}
	return "fr"
}

func randomDomain() string {
	domains := []string{"gmail.com", "outlook.com", "videotron.ca", "bell.net", "hotmail.com", "yahoo.ca"}
	return pick(domains)
}

// generateRAMQ generates a realistic RAMQ number.
// Format: AAAA NNNN NNNN (12 chars)
// - 3 first letters of last name (uppercased, unaccented)
// - 1 first letter of first name (uppercased, unaccented)
// - 2 digits: year of birth (+ 50 for females)
// - 2 digits: month of birth
// - 2 digits: day of birth
// - 2 digits: sequence (random 01-99)
func generateRAMQ(firstName, lastName string, dob time.Time, isFemale bool) string {
	ln := upper(lastName)
	fn := upper(firstName)

	// Pad last name to at least 3 chars
	for len(ln) < 3 {
		ln += "X"
	}

	year := dob.Year() % 100
	month := int(dob.Month())
	if isFemale {
		month += 50
	}

	seq := rand.Intn(99) + 1

	return fmt.Sprintf("%s%s %02d%02d %02d%02d",
		ln[:3], fn[:1],
		year, month,
		dob.Day(), seq,
	)
}

// seedExternalSystems creates external systems and assigns external IDs to some participants.
func seedExternalSystems(db *gorm.DB) {
	cqdg := types.ExternalSystem{
		Name:    "CQDG",
		TitleFr: "Centre québécois de données génomiques",
		TitleEn: "Quebec Centre for Genomic Data",
	}
	db.Where("name = ?", cqdg.Name).FirstOrCreate(&cqdg)

	cqgc := types.ExternalSystem{
		Name:    "CQGC",
		TitleFr: "Centre québécois de génomique clinique",
		TitleEn: "Quebec Centre for Clinical Genomics",
	}
	db.Where("name = ?", cqgc.Name).FirstOrCreate(&cqgc)

	var participants []types.Participant
	db.Find(&participants)

	cqdgCount, cqgcCount := 0, 0
	for _, p := range participants {
		if rand.Float64() < 0.6 {
			db.Create(&types.ExternalID{
				ExternalSystemID: cqdg.ID,
				ParticipantID:    p.ID,
				ExternalID:       fmt.Sprintf("CQDG-%06d", rand.Intn(999999)+1),
			})
			cqdgCount++
		}
		if rand.Float64() < 0.4 {
			db.Create(&types.ExternalID{
				ExternalSystemID: cqgc.ID,
				ParticipantID:    p.ID,
				ExternalID:       fmt.Sprintf("CQGC-%06d", rand.Intn(999999)+1),
			})
			cqgcCount++
		}
	}

	log.Printf("External systems seeded: CQDG (%d IDs), CQGC (%d IDs)", cqdgCount, cqgcCount)
}

func upper(s string) string {
	result := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			result = append(result, byte(r-32))
		case r >= 'A' && r <= 'Z':
			result = append(result, byte(r))
		case r == 'é' || r == 'è' || r == 'ê' || r == 'ë' || r == 'É':
			result = append(result, 'E')
		case r == 'à' || r == 'â' || r == 'ä':
			result = append(result, 'A')
		case r == 'î' || r == 'ï':
			result = append(result, 'I')
		case r == 'ô' || r == 'ö':
			result = append(result, 'O')
		case r == 'ù' || r == 'û' || r == 'ü':
			result = append(result, 'U')
		case r == 'ç':
			result = append(result, 'C')
		}
	}
	return string(result)
}
