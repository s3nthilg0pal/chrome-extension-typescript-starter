# Media Name Extractor API

A Python API that extracts movie and TV show names from torrent filenames using Ollama with the gemma3 model.

## Requirements

- Python 3.11+
- Ollama running with `gemma3` model

## Installation

```bash
pip install -r requirements.txt
```

## Configuration

Set environment variables (optional):

```bash
export OLLAMA_HOST="http://localhost:11434"  # Default
export OLLAMA_MODEL="gemma3"                  # Default
```

## Running the API

```bash
python main.py
```

Or with uvicorn directly:

```bash
uvicorn main:app --host 0.0.0.0 --port 8000 --reload
```

## API Endpoints

### GET `/extract`

Extract media name from a torrent string.

**Query Parameters:**
- `q` (required): The torrent filename or link

**Example Request:**
```bash
curl "http://localhost:8000/extract?q=The.Matrix.1999.1080p.BluRay.x264-GROUP"
```

**Example Response:**
```json
{
  "original_input": "The.Matrix.1999.1080p.BluRay.x264-GROUP",
  "extracted_name": "The Matrix",
  "year": "1999",
  "media_type": "movie"
}
```

### GET `/health`

Check API and Ollama connection status.

### GET `/`

Basic health check.

## API Documentation

Once running, visit:
- Swagger UI: http://localhost:8000/docs
- ReDoc: http://localhost:8000/redoc

## Usage in Other Services

```python
import requests

response = requests.get(
    "http://localhost:8000/extract",
    params={"q": "Inception.2010.2160p.UHD.BluRay"}
)
data = response.json()
print(data["extracted_name"])  # "Inception"
```

```javascript
// JavaScript/Node.js
const response = await fetch(
  "http://localhost:8000/extract?q=Inception.2010.2160p.UHD.BluRay"
);
const data = await response.json();
console.log(data.extracted_name);  // "Inception"
```
