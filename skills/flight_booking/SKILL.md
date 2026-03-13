---
name: flight_booking
description: Use this skill to help users book flight tickets with intelligent flight selection
license: MIT
tags:
  - travel
  - booking
  - flights
memory:
  preferred_airline: Store the user's preferred airline for future bookings
  frequent_routes: Remember routes the user books frequently
---

# Flight Booking Expert

This skill guides you through helping users book flight tickets efficiently.

## When to Use This Skill

Use this skill whenever a user wants to:
- Book a flight ticket
- Find and reserve flights
- Search for flights and make a reservation
- Plan travel with flight bookings

## Available Tools

You have access to these tools:
- `flight_search` - Search for available flights between two cities
- `flight_reservation` - Reserve a specific flight for a passenger

## Workflow

Follow this process when booking flights:

### 1. Gather Information

First, collect these details from the user:
- **Departure city** (from)
- **Destination city** (to)
- **Travel date**
- **Passenger name**
- **Preferences** (optional: preferred airline, budget constraints)

### 2. Search for Flights

Use the `flight_search` tool with the route and date:
```
<tool_call>{"tool_name": "flight_search", "input": {"from": "New York", "to": "London", "date": "2026-03-15"}}</tool_call>
```

### 3. Analyze Options

When you receive flight options:
- **Check user preferences**: If the user has a preferred airline in memory, prioritize those flights
- **Consider price**: If the user mentioned budget constraints, filter accordingly
- **Evaluate timing**: Consider departure/arrival times that might suit the user

### 4. Select Best Flight

Choose the most appropriate flight based on:
1. **Preference match**: Preferred airline if specified
2. **Best value**: Lowest price among suitable options
3. **Convenience**: Reasonable departure times

### 5. Make Reservation

Once you've selected a flight, use the `flight_reservation` tool:
```
<tool_call>{"tool_name": "flight_reservation", "input": {"flight_number": "DL303", "passenger_name": "John Doe"}}</tool_call>
```

### 6. Confirm with User

After successful reservation:
- Share the confirmation number
- Summarize flight details (flight number, airline, time, price)
- Ask if they need anything else

## Memory Integration

**Remember to use and update long-term memory:**

- **Before searching**: Check memory for `preferred_airline` and use it in selection
- **After booking**: Store the airline used if the user seems satisfied
- **Track patterns**: Note if the user books the same route multiple times

Example memory operations:
- Retrieve: "Let me check your preferred airline..." (query memory)
- Store: "I'll remember you prefer Delta for future bookings" (after successful booking)

## Error Handling

If a tool fails:
- **Flight search fails**: Ask user to try different dates or nearby airports
- **Reservation fails**: Check if flight is fully booked, suggest alternatives
- **Missing information**: Politely ask for required details

## Example Interaction

**User**: "Book a flight from NYC to London for tomorrow"

**Your process**:
1. Check memory for preferred airline
2. Use flight_search with NYC → London and tomorrow's date
3. Analyze results, prioritize preferred airline if found in memory
4. Select best flight (e.g., Delta for lowest price)
5. Use flight_reservation with selected flight and user name from memory
6. Confirm: "I've booked you on Delta flight DL303 departing 7:00 PM. Confirmation: CONF-12345. This matches your preferred airline Delta."

## Tips

- Always explain your reasoning when selecting flights
- Be proactive about using memory to personalize the experience
- If unsure about preferences, ask rather than assume
- Provide clear, structured confirmation details
