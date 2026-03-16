#!/bin/bash

# Example: Using Subagents with MachineSpirit
# This script demonstrates how to interact with the subagent feature

echo "================================================"
echo "MachineSpirit Subagent Example"
echo "================================================"
echo ""
echo "This example shows how to use subagents to delegate tasks."
echo ""
echo "Prerequisites:"
echo "  1. Build MachineSpirit: go build ./cmd/ms"
echo "  2. Set up your LLM provider (OpenAI or Anthropic)"
echo "  3. Set API key: export OPENAI_API_KEY=your-key"
echo ""
echo "================================================"
echo ""

# Create a test workspace
WORKSPACE_DIR="/tmp/ms-subagent-example"
mkdir -p "$WORKSPACE_DIR"

echo "Created test workspace: $WORKSPACE_DIR"
echo ""

# Create some sample files for the subagent to work with
cat > "$WORKSPACE_DIR/sample.txt" << 'EOF'
Line 1: This is a sample file.
Line 2: It contains some text data.
Line 3: The subagent can read and analyze this.
Line 4: Total word count should be calculated.
Line 5: End of sample data.
EOF

echo "Created sample.txt with test data"
echo ""

# Create a task file with example interactions
cat > "$WORKSPACE_DIR/example-tasks.txt" << 'EOF'
Example Tasks for Subagent:

1. File Analysis Task:
   "Please create a subagent to analyze the file sample.txt and count the number of lines and words."

2. Data Processing Task:
   "Use a subagent to read sample.txt and extract all lines containing the word 'data'."

3. Multi-Step Task:
   "Delegate to a subagent: read sample.txt, count words, and write the count to results.txt."

When prompted, you'll need to approve each subagent action.
EOF

echo "================================================"
echo "Example Interactions"
echo "================================================"
echo ""
cat "$WORKSPACE_DIR/example-tasks.txt"
echo ""
echo "================================================"
echo "Starting MachineSpirit"
echo "================================================"
echo ""
echo "When the prompt appears, try one of the example tasks above."
echo "You'll see approval prompts for subagent actions."
echo ""
echo "Example session:"
echo "  > Use a subagent to analyze sample.txt"
echo "  [You'll be asked to approve the subagent creation]"
echo "  [You'll be asked to approve each tool the subagent uses]"
echo "  [You'll see real-time progress updates]"
echo "  [Finally, you'll get the results]"
echo ""

# Check if the binary exists
if [ ! -f "./ms" ]; then
    echo "Error: MachineSpirit binary not found!"
    echo "Please build it first: go build ./cmd/ms"
    exit 1
fi

# Check if API key is set
if [ -z "$OPENAI_API_KEY" ] && [ -z "$ANTHROPIC_API_KEY" ]; then
    echo "Warning: No API key found!"
    echo "Please set OPENAI_API_KEY or ANTHROPIC_API_KEY"
    echo ""
    echo "Example:"
    echo "  export OPENAI_API_KEY=your-key-here"
    echo "  ./examples/subagent-example.sh"
    exit 1
fi

# Run MachineSpirit with the test workspace
echo "Running: ./ms --workspace $WORKSPACE_DIR"
echo ""
./ms --workspace "$WORKSPACE_DIR"

# Cleanup
echo ""
echo "================================================"
echo "Cleanup"
echo "================================================"
read -p "Remove test workspace? (y/n): " cleanup
if [ "$cleanup" = "y" ]; then
    rm -rf "$WORKSPACE_DIR"
    echo "Removed $WORKSPACE_DIR"
else
    echo "Workspace preserved at: $WORKSPACE_DIR"
fi
