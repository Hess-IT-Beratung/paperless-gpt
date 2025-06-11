
# Paperless-GPT

This is a fork of the [paperless-gpt](https://github.com/icereed/paperless-gpt)

A Go application that automatically processes and tags documents in Paperless-NGX using AI. It monitors documents with specific tags and applies AI-generated suggestions for titles, tags, correspondents, and document types. The application also supports OCR processing via AWS Textract.

## Prerequisites

- Go 1.23 or later
- Access to a Paperless-NGX instance
- OpenAI API key or Ollama instance
- AWS credentials (for OCR functionality)

## Local Development

### 1. Clone and Setup

```bash
git clone <repository-url>
cd paperless-gpt
```

### 2. Environment Configuration

Create a `.env` file in the root directory with the following variables:

```bash
# Required
PAPERLESS_BASE_URL="http://localhost:8000"
PAPERLESS_API_TOKEN="your-paperless-api-token"
LLM_PROVIDER="openai"  # or "ollama"
LLM_MODEL="gpt-4o"     # or your preferred model
OPENAI_API_KEY="your-openai-api-key"

# AWS (required for OCR)
AWS_ACCESS_KEY_ID="your-aws-access-key"
AWS_SECRET_ACCESS_KEY="your-aws-secret-key"
AWS_REGION="eu-central-1"
AWS_OCR_BUCKET_NAME="your-ocr-bucket"

# Optional (with defaults)
PAPERLESS_AUTO_TAG="paperless-gpt-auto"
PAPERLESS_OCR_TAG="paperless-gpt-ocr"
LOG_LEVEL="debug"
LLM_LANGUAGE="English"
PROMPTS_DIR="./internal/config/prompts"
```

### 3. Install Dependencies

```bash
go mod download
go mod tidy
```

### 4. Build the Application

```bash
# Build binary
go build -o paperless-gpt ./cmd/paperless-gpt

# Or build and run directly
go run ./cmd/paperless-gpt
```

### 5. Start the Application

#### Option A: Using environment variables from .env file

```bash
# Load environment variables and run
set -a && source .env && set +a && go run ./cmd/paperless-gpt
```

#### Option B: Run the compiled binary

```bash
# Load environment variables and run binary
set -a && source .env && set +a && ./paperless-gpt
```

#### Option C: Using Docker

```bash
# Build Docker image
docker build -t paperless-gpt .

# Run with environment file
docker run --env-file .env paperless-gpt
```

## How It Works

The application runs two concurrent processes:

1. **Auto-tagging**: Monitors documents with `PAPERLESS_AUTO_TAG` and generates AI suggestions
2. **OCR Processing**: Monitors documents with `PAPERLESS_OCR_TAG` and performs AWS Textract OCR

To use the application:

1. Tag documents in Paperless-NGX with the configured tag names
2. The application will automatically process them and update with AI suggestions
3. Original tags are removed after processing

## Configuration

See `internal/config/Env.go` for all available environment variables and their defaults.