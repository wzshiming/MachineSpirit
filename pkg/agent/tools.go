package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// FlightSearchTool searches for available flights.
type FlightSearchTool struct{}

// NewFlightSearchTool creates a new flight search tool.
func NewFlightSearchTool() *FlightSearchTool {
	return &FlightSearchTool{}
}

func (t *FlightSearchTool) Name() string {
	return "flight_search"
}

func (t *FlightSearchTool) Description() string {
	return "Search for available flights between two cities on a specific date. Returns a list of flights with prices and times."
}

func (t *FlightSearchTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"from": map[string]interface{}{
				"type":        "string",
				"description": "Departure city",
			},
			"to": map[string]interface{}{
				"type":        "string",
				"description": "Destination city",
			},
			"date": map[string]interface{}{
				"type":        "string",
				"description": "Travel date in YYYY-MM-DD format",
			},
		},
		"required": []string{"from", "to", "date"},
	}
}

func (t *FlightSearchTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		From string `json:"from"`
		To   string `json:"to"`
		Date string `json:"date"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if params.From == "" || params.To == "" || params.Date == "" {
		return "", fmt.Errorf("from, to, and date are required")
	}

	// Simulate flight search (in real implementation, this would call an API)
	flights := []map[string]interface{}{
		{
			"flight_number": "AA101",
			"airline":       "American Airlines",
			"departure":     "08:00",
			"arrival":       "11:30",
			"price":         "$450",
			"duration":      "3h 30m",
		},
		{
			"flight_number": "BA202",
			"airline":       "British Airways",
			"departure":     "14:00",
			"arrival":       "17:45",
			"price":         "$520",
			"duration":      "3h 45m",
		},
		{
			"flight_number": "DL303",
			"airline":       "Delta Airlines",
			"departure":     "19:00",
			"arrival":       "22:20",
			"price":         "$380",
			"duration":      "3h 20m",
		},
	}

	result, err := json.Marshal(map[string]interface{}{
		"from":    params.From,
		"to":      params.To,
		"date":    params.Date,
		"flights": flights,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	return string(result), nil
}

// FlightReservationTool makes flight reservations.
type FlightReservationTool struct{}

// NewFlightReservationTool creates a new flight reservation tool.
func NewFlightReservationTool() *FlightReservationTool {
	return &FlightReservationTool{}
}

func (t *FlightReservationTool) Name() string {
	return "flight_reservation"
}

func (t *FlightReservationTool) Description() string {
	return "Reserve a specific flight. Returns a confirmation number if successful."
}

func (t *FlightReservationTool) ParametersSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"flight_number": map[string]interface{}{
				"type":        "string",
				"description": "The flight number to reserve",
			},
			"passenger_name": map[string]interface{}{
				"type":        "string",
				"description": "Full name of the passenger",
			},
		},
		"required": []string{"flight_number", "passenger_name"},
	}
}

func (t *FlightReservationTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		FlightNumber  string `json:"flight_number"`
		PassengerName string `json:"passenger_name"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	if params.FlightNumber == "" || params.PassengerName == "" {
		return "", fmt.Errorf("flight_number and passenger_name are required")
	}

	// Simulate reservation (in real implementation, this would call an API)
	// Simulate occasional failures for testing feedback loops
	if params.FlightNumber == "XX999" {
		return "", fmt.Errorf("flight %s is fully booked", params.FlightNumber)
	}

	confirmationNumber := fmt.Sprintf("CONF-%d", time.Now().Unix()%100000)

	result, err := json.Marshal(map[string]interface{}{
		"status":              "confirmed",
		"confirmation_number": confirmationNumber,
		"flight_number":       params.FlightNumber,
		"passenger_name":      params.PassengerName,
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	return string(result), nil
}
