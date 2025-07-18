package repository

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/database"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/models"
)

type leadRepository struct {
	db *sql.DB
}

var leadRepo *leadRepository

// GetLeadRepository returns lead repository instance
func GetLeadRepository() *leadRepository {
	if leadRepo == nil {
		leadRepo = &leadRepository{
			db: database.GetDB(),
		}
	}
	return leadRepo
}

// CreateLead creates a new lead
func (r *leadRepository) CreateLead(lead *models.Lead) error {
	// Let database generate the ID (SERIAL)
	lead.CreatedAt = time.Now()
	lead.UpdatedAt = time.Now()

	query := `
		INSERT INTO leads (device_id, user_id, name, phone, niche, journey, status, target_status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	
	// Map Notes to journey column
	journey := lead.Notes
	
	// Default target_status if empty
	if lead.TargetStatus == "" {
		lead.TargetStatus = "prospect"
	}
	
	// Status can be empty or use a default
	status := lead.Status
	if status == "" {
		status = "new"  // Default status
	}
	
	var id int
	err := r.db.QueryRow(query, lead.DeviceID, lead.UserID, lead.Name, lead.Phone, 
		lead.Niche, journey, status, lead.TargetStatus, lead.CreatedAt, lead.UpdatedAt).Scan(&id)
	
	if err == nil {
		lead.ID = fmt.Sprintf("%d", id)
	}
		
	return err
}

// GetLeadsByNiche gets all leads matching a niche (supports comma-separated niches)
func (r *leadRepository) GetLeadsByNiche(niche string) ([]models.Lead, error) {
	// Use LIKE pattern to match leads that contain this niche
	// This will match:
	// - Exact match: niche = 'ITADRESS'
	// - As first item: niche = 'ITADRESS,OTHER'
	// - As middle item: niche = 'OTHER,ITADRESS,MORE'
	// - As last item: niche = 'OTHER,ITADRESS'
	query := `
		SELECT id, device_id, user_id, name, phone, niche, journey, status, 
		       COALESCE(target_status, 'prospect') as target_status, created_at, updated_at
		FROM leads
		WHERE niche = $1 
		   OR niche LIKE $2 
		   OR niche LIKE $3 
		   OR niche LIKE $4
		ORDER BY created_at DESC
	`
	
	// Pattern matching for comma-separated values
	exactMatch := niche
	startsWithPattern := niche + ",%"
	endsWithPattern := "%," + niche
	containsPattern := "%," + niche + ",%"
	
	rows, err := r.db.Query(query, exactMatch, startsWithPattern, endsWithPattern, containsPattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var leads []models.Lead
	for rows.Next() {
		var lead models.Lead
		var journey sql.NullString
		err := rows.Scan(&lead.ID, &lead.DeviceID, &lead.UserID, &lead.Name, &lead.Phone,
			&lead.Niche, &journey, &lead.Status, &lead.TargetStatus,
			&lead.CreatedAt, &lead.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning lead in GetLeadsByNiche: %v", err)
			continue
		}
		// Map journey to Notes field
		if journey.Valid {
			lead.Notes = journey.String
		}
		leads = append(leads, lead)
	}
	
	return leads, nil
}

// GetLeadsByNicheAndStatus gets all leads matching a niche AND status
func (r *leadRepository) GetLeadsByNicheAndStatus(niche string, status string) ([]models.Lead, error) {
	// First, get all leads matching the niche
	leads, err := r.GetLeadsByNiche(niche)
	if err != nil {
		return nil, err
	}
	
	// If no status specified, return all leads matching the niche
	if status == "" {
		return leads, nil
	}
	
	// Filter by target_status
	var filteredLeads []models.Lead
	for _, lead := range leads {
		if lead.TargetStatus == status {
			filteredLeads = append(filteredLeads, lead)
		}
	}
	
	return filteredLeads, nil
}

// GetNewLeadsForSequence gets new leads matching niche that aren't in sequence
func (r *leadRepository) GetNewLeadsForSequence(niche, sequenceID string) ([]models.Lead, error) {
	query := `
		SELECT l.id, l.user_id, l.name, l.phone, l.niche, 
		       l.journey, l.status, l.created_at, l.updated_at
		FROM leads l
		WHERE l.niche = $1
		AND NOT EXISTS (
			SELECT 1 FROM sequence_contacts sc 
			WHERE sc.sequence_id = $2 
			AND sc.contact_phone = l.phone
		)
		ORDER BY l.created_at DESC
	`
	
	rows, err := r.db.Query(query, niche, sequenceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var leads []models.Lead
	for rows.Next() {
		var lead models.Lead
		var journey sql.NullString
		err := rows.Scan(&lead.ID, &lead.UserID, &lead.Name, &lead.Phone,
			&lead.Niche, &journey, &lead.Status,
			&lead.CreatedAt, &lead.UpdatedAt)
		if err != nil {
			log.Printf("Error scanning lead in GetNewLeadsForSequence: %v", err)
			continue
		}
		// Map journey to Notes field
		if journey.Valid {
			lead.Notes = journey.String
		}
		leads = append(leads, lead)
	}
	
	return leads, nil
}

// GetLeadsByDevice gets all leads for a specific user's device
func (r *leadRepository) GetLeadsByDevice(userID, deviceID string) ([]models.Lead, error) {
	query := `
		SELECT id, device_id, user_id, name, phone, niche, journey, status, target_status, created_at, updated_at
		FROM leads
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var leads []models.Lead
	for rows.Next() {
		var lead models.Lead
		var journey sql.NullString
		var targetStatus sql.NullString
		
		err := rows.Scan(&lead.ID, &lead.DeviceID, &lead.UserID, &lead.Name, &lead.Phone,
			&lead.Niche, &journey, &lead.Status, &targetStatus,
			&lead.CreatedAt, &lead.UpdatedAt)
		if err != nil {
			continue
		}
		
		// Map journey to Notes field
		if journey.Valid {
			lead.Notes = journey.String
		}
		
		// Map target_status
		if targetStatus.Valid {
			lead.TargetStatus = targetStatus.String
		}
		
		leads = append(leads, lead)
	}
	
	return leads, nil
}

// UpdateLead updates an existing lead
func (r *leadRepository) UpdateLead(id string, lead *models.Lead) error {
	lead.UpdatedAt = time.Now()
	
	query := `
		UPDATE leads 
		SET device_id = $2, name = $3, phone = $4, niche = $5, 
		    journey = $6, status = $7, target_status = $8, updated_at = $9
		WHERE id = $1
	`
	
	// Map Notes to journey column
	journey := lead.Notes
	
	// Default status if empty
	status := lead.Status
	if status == "" {
		status = "new"
	}
	
	// Default target_status if empty
	if lead.TargetStatus == "" {
		lead.TargetStatus = "prospect"
	}
	
	result, err := r.db.Exec(query, id, lead.DeviceID, lead.Name, lead.Phone,
		lead.Niche, journey, status, lead.TargetStatus, lead.UpdatedAt)
	if err != nil {
		return err
	}
	
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("lead not found")
	}
	
	return nil
}

// DeleteLead deletes a lead
func (r *leadRepository) DeleteLead(id string) error {
	query := `DELETE FROM leads WHERE id = $1`
	
	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}
	
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("lead not found")
	}
	
	return nil
}

// GetLeadsByDeviceNicheAndStatus gets leads for a specific device matching niche and status
func (r *leadRepository) GetLeadsByDeviceNicheAndStatus(deviceID, niche, status string) ([]models.Lead, error) {
	query := `
		SELECT id, device_id, user_id, name, phone, niche, 
		       status, target_status, journey, created_at, updated_at
		FROM leads
		WHERE device_id = $1 
		AND ($2 = '' OR niche LIKE '%' || $2 || '%')
		AND ($3 = '' OR target_status = $3)
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.Query(query, deviceID, niche, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var leads []models.Lead
	for rows.Next() {
		var lead models.Lead
		var journey sql.NullString
		
		err := rows.Scan(
			&lead.ID, &lead.DeviceID, &lead.UserID, &lead.Name, &lead.Phone,
			&lead.Niche, &lead.Status, &lead.TargetStatus, &journey, 
			&lead.CreatedAt, &lead.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		
		// Map journey to Notes field
		if journey.Valid {
			lead.Notes = journey.String
		}
		leads = append(leads, lead)
	}
	
	return leads, nil
}
