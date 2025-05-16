#!/bin/bash

# Script to check if conversation files are properly configured

echo "Checking conversation files..."
echo "=============================="

# Function to check a specific conversation file
check_conversation_file() {
    local mob_file=$1
    local conversation_file=$2
    local mob_name=$3
    
    echo "Checking $mob_name..."
    
    # Check if mob file exists
    if [[ ! -f "$mob_file" ]]; then
        echo "  ERROR: Mob file not found: $mob_file"
        return
    fi
    
    # Check if conversation file exists
    if [[ ! -f "$conversation_file" ]]; then
        echo "  ERROR: Conversation file not found: $conversation_file"
        return
    fi
    
    # Extract mobid from mob file
    mob_id=$(grep -E "^mobid:" "$mob_file" | awk '{print $2}')
    if [[ -z "$mob_id" ]]; then
        echo "  ERROR: Could not extract mobid from $mob_file"
        return
    fi
    
    # Extract name from mob file
    actual_name=$(grep -A 5 "character:" "$mob_file" | grep "name:" | sed 's/.*name: //')
    if [[ -z "$actual_name" ]]; then
        echo "  ERROR: Could not extract name from $mob_file"
        return
    fi
    
    # Check if the conversation file has the right supported entries
    if ! grep -q "\"$actual_name\"" "$conversation_file"; then
        echo "  WARNING: Conversation file doesn't contain the exact mob name: $actual_name"
    fi
    
    # Validate that the conversation file is properly structured
    if ! grep -q "supported:" "$conversation_file"; then
        echo "  ERROR: Missing 'supported:' section in conversation file"
        return
    fi
    
    if ! grep -q "llmconfig:" "$conversation_file"; then
        echo "  ERROR: Missing 'llmconfig:' section in conversation file"
        return
    fi
    
    if ! grep -q "greeting:" "$conversation_file"; then
        echo "  ERROR: Missing 'greeting:' in conversation file"
        return
    fi
    
    if ! grep -q "farewell:" "$conversation_file"; then
        echo "  ERROR: Missing 'farewell:' in conversation file"
        return
    fi
    
    # If we get here, the file looks good
    echo "  GOOD: $conversation_file is properly configured for mob $actual_name (ID: $mob_id)"
}

# Check Moilyn the Wizard
check_conversation_file "_datafiles/world/default/mobs/frostfang/50-moilyn_the_wizard.yaml" \
                        "_datafiles/world/default/conversations/frostfang/50.yaml" \
                        "Moilyn the Wizard"

# Check Elara
check_conversation_file "_datafiles/world/default/mobs/frostfang/39-elara.yaml" \
                        "_datafiles/world/default/conversations/frostfang/39.yaml" \
                        "Elara"

echo "=============================="
echo "Conversation check complete" 