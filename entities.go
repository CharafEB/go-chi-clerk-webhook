package main

import "encoding/json"

// Clerk webhook payload shapes (subset we persist).

type clerkEvent struct {
	Type string `json:"type"`
	Data json.RawMessage `json:"data"`
}

type clerkEmail struct {
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
}

type clerkPhone struct {
	ID          string `json:"id"`
	PhoneNumber string `json:"phone_number"`
}

type clerkUser struct {
	ID                    string       `json:"id"`
	FirstName             string       `json:"first_name"`
	LastName              *string      `json:"last_name"`
	Username              *string      `json:"username"`
	ImageURL              string       `json:"image_url"`
	Gender                *string      `json:"gender"`
	Birthday              *string      `json:"birthday"`
	PasswordEnabled       bool         `json:"password_enabled"`
	TwoFactorEnabled      bool         `json:"two_factor_enabled"`
	PrimaryEmailAddressID string       `json:"primary_email_address_id"`
	PrimaryPhoneNumberID  *string      `json:"primary_phone_number_id"`
	EmailAddresses        []clerkEmail `json:"email_addresses"`
	PhoneNumbers          []clerkPhone `json:"phone_numbers"`
	CreatedAt             int64        `json:"created_at"` // Unix seconds
	UpdatedAt             int64        `json:"updated_at"` // Unix seconds
	LastSignInAt          *int64       `json:"last_sign_in_at"`
}
