from fastapi import FastAPI, Query, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from pydantic import BaseModel
from ollama import Client
import os
import re

app = FastAPI(
    title="Media Name Extractor API",
    description="Extract movie/TV show names from torrent links using AI",
    version="1.0.0"
)

# Add CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Ollama configuration
OLLAMA_HOST = os.getenv("OLLAMA_HOST", "http://192.168.0.162:11434")
MODEL_NAME = os.getenv("OLLAMA_MODEL", "deepseek-r1")

# Initialize Ollama client
client = Client(host=OLLAMA_HOST)


class ExtractionResponse(BaseModel):
    original_input: str
    extracted_name: str
    year: str | None = None
    media_type: str | None = None


def extract_media_name(torrent_string: str) -> dict:
    """Use Ollama to extract movie/TV show name from torrent string."""
    
    prompt = f"""Extract the movie or TV show title from this torrent filename. Return ONLY the clean title as plain text, no formatting, no markdown, no year, no quotes.

Input: {torrent_string}

Title:"""

    try:
        response = client.chat(
            model=MODEL_NAME,
            messages=[
                {
                    "role": "system",
                    "content": "You extract movie/TV show titles from torrent filenames. Respond with ONLY the title as plain text. No markdown, no code blocks, no quotes, no year, no extra text."
                },
                {
                    "role": "user",
                    "content": prompt
                }
            ],
            options={
                "temperature": 0.1  # Low temperature for consistent extraction
            }
        )
        
        result_text = response["message"]["content"].strip()
        
        # Remove markdown code blocks if present
        if "```" in result_text:
            # Extract content between code blocks
            match = re.search(r'```(?:json)?\s*(.*?)\s*```', result_text, re.DOTALL)
            if match:
                result_text = match.group(1).strip()
        
        # Remove any remaining quotes, newlines, parentheses with year
        result_text = result_text.strip('"\'\n')
        result_text = re.sub(r'\s*\(\d{4}\)\s*$', '', result_text)  # Remove (2025) etc.
        result_text = re.sub(r'\s*\d{4}\s*$', '', result_text)  # Remove trailing year
        
        return {
            "name": result_text.strip(),
            "year": None,
            "type": None
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Error communicating with Ollama: {str(e)}")


@app.get("/", tags=["Health"])
async def root():
    """Health check endpoint."""
    return {"status": "ok", "message": "Media Name Extractor API is running"}


@app.get("/extract", response_model=ExtractionResponse, tags=["Extraction"])
async def extract_name(
    q: str = Query(..., description="Torrent filename or link to extract media name from", min_length=1)
):
    """
    Extract movie or TV show name from a torrent filename or link.
    
    **Example queries:**
    - `The.Matrix.1999.1080p.BluRay.x264-GROUP`
    - `Breaking.Bad.S01E01.720p.WEB-DL`
    - `Inception.2010.2160p.UHD.BluRay.REMUX`
    """
    result = extract_media_name(q)
    
    return ExtractionResponse(
        original_input=q,
        extracted_name=result["name"],
        year=result["year"],
        media_type=result["type"]
    )


@app.get("/health", tags=["Health"])
async def health_check():
    """Check if the API and Ollama connection are working."""
    try:
        # Test Ollama connection
        client.list()
        return {
            "status": "healthy",
            "ollama_host": OLLAMA_HOST,
            "model": MODEL_NAME
        }
    except Exception as e:
        raise HTTPException(status_code=503, detail=f"Ollama connection failed: {str(e)}")


if __name__ == "__main__":
    import uvicorn
    uvicorn.run(app, host="0.0.0.0", port=8000)
