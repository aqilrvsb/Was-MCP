package database

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"sync"
	"time"
	
	_ "github.com/lib/pq"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
)

var (
	db   *sql.DB
	once sync.Once
)

// GetDB returns the database connection
func GetDB() *sql.DB {
	once.Do(func() {
		var err error
		
		// Parse PostgreSQL connection string
		db, err = sql.Open("postgres", config.DBURI)
		if err != nil {
			log.Fatalf("Failed to connect to database: %v", err)
		}
		
		// Configure connection pool for 200+ users
		db.SetMaxOpenConns(100)
		db.SetMaxIdleConns(10)
		db.SetConnMaxLifetime(time.Hour)
		
		// Test connection
		if err := db.Ping(); err != nil {
			log.Fatalf("Failed to ping database: %v", err)
		}
		
		// Initialize schema
		if err := InitializeSchema(); err != nil {
			log.Fatalf("Failed to initialize schema: %v", err)
		}
		
		log.Println("Database connection established")
	})
	
	return db
}
// InitializeSchema creates tables if they don't exist
func InitializeSchema() error {
	schema := `
	-- Create extension for UUID generation if not exists
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

	-- Create users table
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		email VARCHAR(255) UNIQUE NOT NULL,
		full_name VARCHAR(255) NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		is_active BOOLEAN DEFAULT true,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_login TIMESTAMP
	);

	-- Create user_devices table
	CREATE TABLE IF NOT EXISTS user_devices (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		device_name VARCHAR(255) NOT NULL,
		phone VARCHAR(50),
		jid VARCHAR(255),
		status VARCHAR(50) DEFAULT 'offline',
		last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, jid)
	);

	-- Create user_sessions table
	CREATE TABLE IF NOT EXISTS user_sessions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		token VARCHAR(255) UNIQUE NOT NULL,
		expires_at TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Create message_analytics table
	CREATE TABLE IF NOT EXISTS message_analytics (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		device_id UUID REFERENCES user_devices(id) ON DELETE SET NULL,
		message_id VARCHAR(255) NOT NULL,
		jid VARCHAR(255) NOT NULL,
		content TEXT,
		is_from_me BOOLEAN DEFAULT false,
		status VARCHAR(50) NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(message_id)
	);

	-- Create leads table
	CREATE TABLE IF NOT EXISTS leads (
		id SERIAL PRIMARY KEY,
		device_id UUID NOT NULL,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		phone VARCHAR(50) NOT NULL,
		niche VARCHAR(255),
		journey TEXT,
		status VARCHAR(50) DEFAULT 'new',
		last_interaction TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Create campaigns table
	CREATE TABLE IF NOT EXISTS campaigns (
		id SERIAL PRIMARY KEY,
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		campaign_date DATE NOT NULL,
		title VARCHAR(255) NOT NULL,
		niche VARCHAR(255),
		message TEXT NOT NULL,
		image_url TEXT,
		time_schedule TEXT,
		min_delay_seconds INTEGER DEFAULT 10,
		max_delay_seconds INTEGER DEFAULT 30,
		status VARCHAR(50) DEFAULT 'scheduled',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	-- Remove old unique constraint if it exists
	ALTER TABLE campaigns DROP CONSTRAINT IF EXISTS campaigns_user_id_campaign_date_key;
	
	-- Add missing columns to campaigns if they don't exist
	ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS device_id UUID;
	ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS niche VARCHAR(255);
	ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS image_url TEXT;
	ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS target_status VARCHAR(50) DEFAULT 'all';
	ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS time_schedule TEXT;
	ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'scheduled';
	ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS min_delay_seconds INTEGER DEFAULT 10;
	ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS max_delay_seconds INTEGER DEFAULT 30;
	
	-- Add min/max delay columns to user_devices
	ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS min_delay_seconds INTEGER DEFAULT 5;
	ALTER TABLE user_devices ADD COLUMN IF NOT EXISTS max_delay_seconds INTEGER DEFAULT 15;

	-- Create whatsapp_chats table to store chat list
	CREATE TABLE IF NOT EXISTS whatsapp_chats (
		id SERIAL PRIMARY KEY,
		device_id VARCHAR(255) NOT NULL,
		chat_jid VARCHAR(255) NOT NULL,
		chat_name VARCHAR(255) NOT NULL,
		is_group BOOLEAN DEFAULT FALSE,
		is_muted BOOLEAN DEFAULT FALSE,
		last_message_text TEXT,
		last_message_time TIMESTAMP,
		unread_count INTEGER DEFAULT 0,
		avatar_url TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(device_id, chat_jid)
	);

	-- Create whatsapp_messages table to store message history
	CREATE TABLE IF NOT EXISTS whatsapp_messages (
		id SERIAL PRIMARY KEY,
		device_id VARCHAR(255) NOT NULL,
		chat_jid VARCHAR(255) NOT NULL,
		message_id VARCHAR(255) NOT NULL,
		sender_jid VARCHAR(255),
		sender_name VARCHAR(255),
		message_text TEXT,
		message_type VARCHAR(50),
		media_url TEXT,
		is_sent BOOLEAN DEFAULT FALSE,
		is_read BOOLEAN DEFAULT FALSE,
		timestamp TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(device_id, message_id)
	);
	
	-- Create sequences table
	CREATE TABLE IF NOT EXISTS sequences (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		name VARCHAR(255) NOT NULL,
		description TEXT,
		niche VARCHAR(255),
		time_schedule TEXT,
		min_delay_seconds INTEGER DEFAULT 10,
		max_delay_seconds INTEGER DEFAULT 30,
		status VARCHAR(50) DEFAULT 'draft',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	
	-- Create sequence_steps table
	CREATE TABLE IF NOT EXISTS sequence_steps (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		sequence_id UUID NOT NULL REFERENCES sequences(id) ON DELETE CASCADE,
		day_number INTEGER NOT NULL,
		content TEXT,
		image_url TEXT,
		time_schedule TEXT,
		min_delay_seconds INTEGER DEFAULT 5,
		max_delay_seconds INTEGER DEFAULT 15,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(sequence_id, day_number)
	);
	
	-- Create sequence_contacts table
	CREATE TABLE IF NOT EXISTS sequence_contacts (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		sequence_id UUID NOT NULL REFERENCES sequences(id) ON DELETE CASCADE,
		contact_phone VARCHAR(50) NOT NULL,
		contact_name VARCHAR(255),
		current_step INTEGER DEFAULT 0,
		status VARCHAR(50) DEFAULT 'active',
		enrolled_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		last_sent_at TIMESTAMP,
		next_send_at TIMESTAMP,
		completed_at TIMESTAMP,
		UNIQUE(sequence_id, contact_phone)
	);
	
	-- Create broadcast_messages table for tracking
	CREATE TABLE IF NOT EXISTS broadcast_messages (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		device_id UUID NOT NULL REFERENCES user_devices(id) ON DELETE CASCADE,
		campaign_id INTEGER REFERENCES campaigns(id) ON DELETE SET NULL,
		sequence_id UUID REFERENCES sequences(id) ON DELETE SET NULL,
		recipient_phone VARCHAR(50) NOT NULL,
		message_type VARCHAR(50) NOT NULL,
		content TEXT,
		media_url TEXT,
		status VARCHAR(50) DEFAULT 'pending',
		error_message TEXT,
		scheduled_at TIMESTAMP,
		sent_at TIMESTAMP,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		group_id VARCHAR(255),
		group_order INTEGER
	);

	-- Create indexes
	CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
	CREATE INDEX IF NOT EXISTS idx_user_devices_user_id ON user_devices(user_id);
	CREATE INDEX IF NOT EXISTS idx_user_sessions_token ON user_sessions(token);
	CREATE INDEX IF NOT EXISTS idx_message_analytics_user_id ON message_analytics(user_id);
	CREATE INDEX IF NOT EXISTS idx_message_analytics_created_at ON message_analytics(created_at);
	CREATE INDEX IF NOT EXISTS idx_leads_device_id ON leads(device_id);
	CREATE INDEX IF NOT EXISTS idx_leads_user_id ON leads(user_id);
	CREATE INDEX IF NOT EXISTS idx_campaigns_user_id ON campaigns(user_id);
	CREATE INDEX IF NOT EXISTS idx_campaigns_date ON campaigns(campaign_date);
	CREATE INDEX IF NOT EXISTS idx_whatsapp_chats_device_id ON whatsapp_chats(device_id);
	CREATE INDEX IF NOT EXISTS idx_whatsapp_chats_updated ON whatsapp_chats(updated_at DESC);
	CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_device_chat ON whatsapp_messages(device_id, chat_jid);
	CREATE INDEX IF NOT EXISTS idx_whatsapp_messages_timestamp ON whatsapp_messages(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_sequences_user_id ON sequences(user_id);
	CREATE INDEX IF NOT EXISTS idx_sequences_status ON sequences(status);
	CREATE INDEX IF NOT EXISTS idx_sequence_steps_sequence_id ON sequence_steps(sequence_id);
	CREATE INDEX IF NOT EXISTS idx_sequence_contacts_sequence_id ON sequence_contacts(sequence_id);
	CREATE INDEX IF NOT EXISTS idx_sequence_contacts_next_send ON sequence_contacts(next_send_at);
	CREATE INDEX IF NOT EXISTS idx_broadcast_messages_status ON broadcast_messages(status);
	CREATE INDEX IF NOT EXISTS idx_broadcast_messages_scheduled ON broadcast_messages(scheduled_at);
	`
	
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	
	// Add missing columns for sequences (simplified version compatibility)
	alterSchema := `
	-- Add missing columns to sequences table
	ALTER TABLE sequences ADD COLUMN IF NOT EXISTS device_id UUID;
	ALTER TABLE sequences ADD COLUMN IF NOT EXISTS total_days INTEGER DEFAULT 0;
	ALTER TABLE sequences ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true;
	ALTER TABLE sequences ADD COLUMN IF NOT EXISTS time_schedule TEXT;
	
	-- Add missing columns to broadcast_messages if they don't exist
	ALTER TABLE broadcast_messages ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
	ALTER TABLE sequences ADD COLUMN IF NOT EXISTS min_delay_seconds INTEGER DEFAULT 10;
	ALTER TABLE sequences ADD COLUMN IF NOT EXISTS max_delay_seconds INTEGER DEFAULT 30;
	
	-- Make device_id nullable since sequences use all user devices
	ALTER TABLE sequences ALTER COLUMN device_id DROP NOT NULL;

	-- Add missing columns to sequence_steps table
	ALTER TABLE sequence_steps ADD COLUMN IF NOT EXISTS day INTEGER;
	ALTER TABLE sequence_steps ADD COLUMN IF NOT EXISTS send_time VARCHAR(10);
	ALTER TABLE sequence_steps ADD COLUMN IF NOT EXISTS message_type VARCHAR(50) DEFAULT 'text';
	ALTER TABLE sequence_steps ADD COLUMN IF NOT EXISTS media_url TEXT;
	ALTER TABLE sequence_steps ADD COLUMN IF NOT EXISTS caption TEXT;
	ALTER TABLE sequence_steps ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
	ALTER TABLE sequence_steps ADD COLUMN IF NOT EXISTS time_schedule TEXT;

	-- Add missing columns to sequence_contacts table  
	ALTER TABLE sequence_contacts ADD COLUMN IF NOT EXISTS current_day INTEGER DEFAULT 0;
	ALTER TABLE sequence_contacts ADD COLUMN IF NOT EXISTS added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
	ALTER TABLE sequence_contacts ADD COLUMN IF NOT EXISTS last_message_at TIMESTAMP;

	-- Create sequence_logs table if not exists
	CREATE TABLE IF NOT EXISTS sequence_logs (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		sequence_id UUID NOT NULL REFERENCES sequences(id) ON DELETE CASCADE,
		contact_id UUID NOT NULL,
		step_id UUID NOT NULL,
		day INTEGER NOT NULL,
		status VARCHAR(50) NOT NULL,
		message_id VARCHAR(255),
		error_message TEXT,
		sent_at TIMESTAMP NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	-- Create indexes for sequence_logs
	CREATE INDEX IF NOT EXISTS idx_sequence_logs_sequence_id ON sequence_logs(sequence_id);
	CREATE INDEX IF NOT EXISTS idx_sequence_logs_contact_id ON sequence_logs(contact_id);
	CREATE INDEX IF NOT EXISTS idx_sequence_logs_sent_at ON sequence_logs(sent_at);
	
	-- Add group columns to broadcast_messages
	ALTER TABLE broadcast_messages ADD COLUMN IF NOT EXISTS group_id VARCHAR(255);
	ALTER TABLE broadcast_messages ADD COLUMN IF NOT EXISTS group_order INTEGER;
	`
	
	_, err = db.Exec(alterSchema)
	if err != nil {
		log.Printf("Warning: Failed to add sequence columns: %v", err)
		// Don't fail initialization, just log the warning
	}
	
	// Create default admin user if not exists
	var adminExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = 'admin@whatsapp.com')").Scan(&adminExists)
	if err != nil {
		return fmt.Errorf("failed to check admin user: %w", err)
	}
	
	if !adminExists {
		// Encode password with base64 for the default admin
		encodedPassword := base64.StdEncoding.EncodeToString([]byte("changeme123"))
		
		_, err = db.Exec(`
			INSERT INTO users (email, full_name, password_hash, is_active) 
			VALUES ($1, $2, $3, $4)`,
			"admin@whatsapp.com", "Administrator", encodedPassword, true)
		if err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}
		log.Printf("Created default admin user: admin@whatsapp.com / changeme123 (encoded: %s)\n", encodedPassword)
	}
	
	// Run auto-migrations for time_schedule and other updates
	log.Println("Running database migrations...")
	migrations := []struct {
		name string
		sql  string
	}{
		{
			name: "Add target_status columns",
			sql: `
				ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS target_status VARCHAR(50) DEFAULT 'all';
				ALTER TABLE sequences ADD COLUMN IF NOT EXISTS target_status VARCHAR(50) DEFAULT 'all';
				UPDATE campaigns SET target_status = 'all' WHERE target_status IS NULL;
				UPDATE sequences SET target_status = 'all' WHERE target_status IS NULL;
			`,
		},
		{
			name: "Add time_schedule columns",
			sql: `
				-- Add time_schedule to campaigns
				ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS time_schedule TEXT;
				UPDATE campaigns SET time_schedule = scheduled_time WHERE time_schedule IS NULL AND scheduled_time IS NOT NULL;
				
				-- Add time_schedule to sequences
				ALTER TABLE sequences ADD COLUMN IF NOT EXISTS time_schedule TEXT;
				UPDATE sequences SET time_schedule = schedule_time WHERE time_schedule IS NULL AND schedule_time IS NOT NULL;
				
				-- Add time_schedule to sequence_steps
				ALTER TABLE sequence_steps ADD COLUMN IF NOT EXISTS time_schedule TEXT;
				UPDATE sequence_steps SET time_schedule = schedule_time WHERE time_schedule IS NULL AND schedule_time IS NOT NULL;
			`,
		},
		{
			name: "Add scheduled_at for timezone support",
			sql: `
				-- Add TIMESTAMPTZ column for proper timezone support
				ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMPTZ;
				
				-- Migrate existing data to TIMESTAMPTZ (assuming Malaysia timezone)
				UPDATE campaigns 
				SET scheduled_at = 
					CASE 
						WHEN time_schedule IS NULL OR time_schedule = '' THEN 
							campaign_date::TIMESTAMP AT TIME ZONE 'Asia/Kuala_Lumpur'
						ELSE 
							(campaign_date || ' ' || time_schedule)::TIMESTAMP AT TIME ZONE 'Asia/Kuala_Lumpur'
					END
				WHERE scheduled_at IS NULL;
				
				-- Create optimized indexes
				CREATE INDEX IF NOT EXISTS idx_campaigns_scheduled_at ON campaigns(scheduled_at) WHERE status = 'pending';
				CREATE INDEX IF NOT EXISTS idx_campaigns_scheduled_at_status ON campaigns(scheduled_at, status);
			`,
		},
		{
			name: "Add updated_at to broadcast_messages",
			sql: `
				ALTER TABLE broadcast_messages ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
			`,
		},
		{
			name: "Create time validation function",
			sql: `
				CREATE OR REPLACE FUNCTION is_valid_time_schedule(time_str TEXT) 
				RETURNS BOOLEAN AS $$
				BEGIN
					IF time_str IS NULL OR time_str = '' THEN
						RETURN TRUE;
					END IF;
					
					IF time_str ~ '^\d{2}:\d{2}(:\d{2})?$' THEN
						BEGIN
							PERFORM time_str::TIME;
							RETURN TRUE;
						EXCEPTION WHEN OTHERS THEN
							RETURN FALSE;
						END;
					END IF;
					
					RETURN FALSE;
				END;
				$$ LANGUAGE plpgsql;
			`,
		},
		{
			name: "Fix leads table columns",
			sql: `
				-- Ensure target_status exists in leads table
				ALTER TABLE leads ADD COLUMN IF NOT EXISTS target_status VARCHAR(50) DEFAULT 'prospect';
				UPDATE leads SET target_status = 'prospect' WHERE target_status IS NULL;
			`,
		},
	}
	
	for _, migration := range migrations {
		log.Printf("Running migration: %s", migration.name)
		_, err := db.Exec(migration.sql)
		if err != nil {
			log.Printf("Warning: Migration '%s' failed (may already exist): %v", migration.name, err)
		} else {
			log.Printf("✓ Migration '%s' completed successfully", migration.name)
		}
	}
	
	log.Println("All migrations completed")
	
	// Run cleanup for expired sessions
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		
		for range ticker.C {
			db.Exec("DELETE FROM user_sessions WHERE expires_at < CURRENT_TIMESTAMP")
		}
	}()
	
	return nil
}