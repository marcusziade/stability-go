#!/bin/bash

# Get the video ID from file if not provided
if [ -z "$1" ]; then
  if [ -f "output/video_id.txt" ]; then
    ID=$(grep -o "Video ID: [^ ]*" output/video_id.txt | cut -d ' ' -f 3)
    echo "Using ID from file: $ID"
  else
    echo "Error: No video ID provided and no video_id.txt file found."
    exit 1
  fi
else
  ID="$1"
fi

# API Key is required
if [ -z "$2" ]; then
  echo "Error: API key required as second parameter."
  echo "Usage: ./download.sh [video_id] API_KEY"
  exit 1
else
  API_KEY="$2"
fi

echo "Checking video status for ID: $ID"
echo "This will save the video to output/video.mp4 when ready"

# Create output directory if it doesn't exist
mkdir -p output

# Poll until the video is ready
while true; do
  echo -n "."
  
  # Get the status
  STATUS=$(curl -s -H "Authorization: Bearer $API_KEY" \
    -H "Accept: application/json" \
    "https://api.stability.ai/v2beta/image-to-video/result/$ID")
  
  # Check if finished is true
  if echo "$STATUS" | grep -q '"finished":true'; then
    echo
    echo "Video is ready! Downloading..."
    
    # Extract the video data (base64 encoded)
    VIDEO_DATA=$(echo "$STATUS" | grep -o '"video":"[^"]*"' | cut -d '"' -f 4)
    
    if [ -z "$VIDEO_DATA" ]; then
      echo "Error: No video data found in the response."
      exit 1
    fi
    
    # Save the base64 data to a temporary file
    echo "$VIDEO_DATA" > output/video_base64.txt
    
    # Decode and save the video
    cat output/video_base64.txt | base64 -d > output/video.mp4
    
    echo "Video saved to output/video.mp4"
    echo "Cleaning up temporary files..."
    rm output/video_base64.txt
    
    # Get file size
    SIZE=$(ls -lh output/video.mp4 | awk '{print $5}')
    echo "Video size: $SIZE"
    break
  elif echo "$STATUS" | grep -q "error"; then
    echo
    echo "Error from API:"
    echo "$STATUS"
    exit 1
  fi
  
  # Wait before next check
  sleep 5
done