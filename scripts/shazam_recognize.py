import asyncio
import sys
import json
from shazamio import Shazam

async def main():
    if len(sys.argv) < 2:
        print(json.dumps({"matched": False, "error": "missing snippet path"}))
        sys.exit(1)

    snippet_path = sys.argv[1]
    shazam = Shazam()
    
    try:
        out = await shazam.recognize(snippet_path)
        track = out.get("track")
        if not track:
            print(json.dumps({"matched": False}))
            return

        title = track.get("title", "")
        artist = track.get("subtitle", "")
        
        # Extract external streaming links if available
        spotify_url = ""
        youtube_url = ""
        for provider in track.get("hub", {}).get("providers", []):
            if provider.get("type") == "SPOTIFY":
                actions = provider.get("actions", [])
                if actions:
                    spotify_url = actions[0].get("uri", "")

        result = {
            "matched": True,
            "title": title,
            "artist": artist,
            "spotify_url": spotify_url,
            "youtube_url": youtube_url
        }
        print(json.dumps(result))
    except Exception as e:
        print(json.dumps({"matched": False, "error": str(e)}))

if __name__ == "__main__":
    asyncio.run(main())
