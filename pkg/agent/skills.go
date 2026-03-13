package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// FlightBookingSkill is a high-level skill that composes flight search and reservation.
type FlightBookingSkill struct {
	searchTool      Tool
	reservationTool Tool
}

// NewFlightBookingSkill creates a new flight booking skill.
func NewFlightBookingSkill(searchTool, reservationTool Tool) *FlightBookingSkill {
	return &FlightBookingSkill{
		searchTool:      searchTool,
		reservationTool: reservationTool,
	}
}

func (s *FlightBookingSkill) Name() string {
	return "flight_booking"
}

func (s *FlightBookingSkill) Description() string {
	return "End-to-end flight booking with intelligent flight selection based on preferences"
}

func (s *FlightBookingSkill) DetailedDescription() string {
	return `This skill handles the complete flight booking workflow:
1. Searches for available flights between origin and destination
2. Analyzes options based on user preferences (price, airline, time)
3. Automatically selects the best flight
4. Makes the reservation

It composes the flight_search and flight_reservation tools with intelligent
decision-making logic to provide a seamless booking experience.

Example usage:
- "Book a flight from New York to London for tomorrow"
- "Find and book the cheapest flight to Paris next week"
- "Reserve a Delta flight from LA to NYC on Friday"`
}

func (s *FlightBookingSkill) ParametersSchema() map[string]interface{} {
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
			"passenger_name": map[string]interface{}{
				"type":        "string",
				"description": "Full name of the passenger",
			},
			"preferred_airline": map[string]interface{}{
				"type":        "string",
				"description": "Preferred airline (optional)",
			},
			"max_price": map[string]interface{}{
				"type":        "string",
				"description": "Maximum acceptable price (optional)",
			},
		},
		"required": []string{"from", "to", "date", "passenger_name"},
	}
}

func (s *FlightBookingSkill) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		From             string `json:"from"`
		To               string `json:"to"`
		Date             string `json:"date"`
		PassengerName    string `json:"passenger_name"`
		PreferredAirline string `json:"preferred_airline"`
		MaxPrice         string `json:"max_price"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}

	// Step 1: Search for flights
	searchInput, _ := json.Marshal(map[string]string{
		"from": params.From,
		"to":   params.To,
		"date": params.Date,
	})

	searchResult, err := s.searchTool.Execute(ctx, searchInput)
	if err != nil {
		return "", fmt.Errorf("flight search failed: %w", err)
	}

	// Step 2: Parse search results and select best flight
	var searchData struct {
		Flights []map[string]interface{} `json:"flights"`
	}
	if err := json.Unmarshal([]byte(searchResult), &searchData); err != nil {
		return "", fmt.Errorf("failed to parse search results: %w", err)
	}

	if len(searchData.Flights) == 0 {
		return "", fmt.Errorf("no flights found")
	}

	// Intelligent flight selection based on preferences
	selectedFlight := s.selectBestFlight(searchData.Flights, params.PreferredAirline, params.MaxPrice)

	// Step 3: Make reservation
	reservationInput, _ := json.Marshal(map[string]string{
		"flight_number":  selectedFlight["flight_number"].(string),
		"passenger_name": params.PassengerName,
	})

	reservationResult, err := s.reservationTool.Execute(ctx, reservationInput)
	if err != nil {
		return "", fmt.Errorf("reservation failed: %w", err)
	}

	// Step 4: Format comprehensive response
	response := map[string]interface{}{
		"status":             "success",
		"selected_flight":    selectedFlight,
		"reservation_result": json.RawMessage(reservationResult),
	}

	result, _ := json.Marshal(response)
	return string(result), nil
}

func (s *FlightBookingSkill) selectBestFlight(flights []map[string]interface{}, preferredAirline, maxPrice string) map[string]interface{} {
	// Simple selection logic: prefer airline if specified, otherwise cheapest
	var bestFlight map[string]interface{}

	for _, flight := range flights {
		if bestFlight == nil {
			bestFlight = flight
			continue
		}

		// Prefer flights from preferred airline
		if preferredAirline != "" {
			if airline, ok := flight["airline"].(string); ok && airline == preferredAirline {
				bestFlight = flight
				break
			}
		}

		// Otherwise select cheapest (simplified comparison)
		currentPrice := flight["price"].(string)
		bestPrice := bestFlight["price"].(string)
		if currentPrice < bestPrice {
			bestFlight = flight
		}
	}

	return bestFlight
}

// ToolAsSkill wraps a Tool to be used as a Skill.
type ToolAsSkill struct {
	tool Tool
}

// NewToolAsSkill wraps a tool to be used as a skill.
func NewToolAsSkill(tool Tool) *ToolAsSkill {
	return &ToolAsSkill{tool: tool}
}

func (s *ToolAsSkill) Name() string {
	return s.tool.Name()
}

func (s *ToolAsSkill) Description() string {
	return s.tool.Description()
}

func (s *ToolAsSkill) DetailedDescription() string {
	// Tools don't have detailed descriptions, so return the same as Description
	return s.tool.Description()
}

func (s *ToolAsSkill) ParametersSchema() map[string]interface{} {
	return s.tool.ParametersSchema()
}

func (s *ToolAsSkill) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	return s.tool.Execute(ctx, input)
}
