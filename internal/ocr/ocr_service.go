package ocr

import (
	"bytes"
	"container/list"
	"context"
	"fmt"
	"paperless-gpt/internal/config"
	"paperless-gpt/internal/logging"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	aws_config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/textract"
	"github.com/aws/aws-sdk-go-v2/service/textract/types"
)

var (
	log = logging.InitLogger(config.LogLevel)
)

const maxCacheSize = 100

type CacheEntry struct {
	key   int
	value string
}

type Cache struct {
	cacheMap  map[int]string
	cacheList *list.List
	mutex     sync.Mutex
}

var ocrCache = Cache{
	cacheMap:  make(map[int]string),
	cacheList: list.New(),
}

func ProcessDocumentOcr(docBytes []byte, documentId int) (string, error) {
	ocrCache.mutex.Lock()
	if cachedResult, found := ocrCache.cacheMap[documentId]; found {
		ocrCache.mutex.Unlock()
		return cachedResult, nil
	}
	ocrCache.mutex.Unlock()

	// Load AWS configuration
	awsConfig, err := aws_config.LoadDefaultConfig(context.TODO(), aws_config.WithRegion(config.Region))
	if err != nil {
		return "", fmt.Errorf("unable to load SDK config: %v", err)
	}

	// Set up clients for S3 and Textract
	s3Client := s3.NewFromConfig(awsConfig)
	textractClient := textract.NewFromConfig(awsConfig)

	objectKey := fmt.Sprintf("uploaded-pdf-%d-%s.pdf", documentId, time.Now().Format("20060102-150405"))

	// Upload the document to S3
	if err := uploadToS3(s3Client, config.Bucket, objectKey, docBytes); err != nil {
		return "", fmt.Errorf("failed to upload document to S3: %v", err)
	}
	log.Infof("Successfully uploaded document to S3 with key: %s", objectKey)

	// Ensure the file is deleted from S3 after processing
	defer func() {
		if err := deleteFromS3(s3Client, config.Bucket, objectKey); err != nil {
			log.Errorf("failed to delete document from S3: %v", err)
		} else {
			log.Infof("Successfully deleted document from S3 with key: %s", objectKey)
		}
	}()

	// Start OCR job on Textract
	jobID, err := startDocumentTextDetection(textractClient, config.Bucket, objectKey)
	if err != nil {
		return "", fmt.Errorf("failed to start text detection job: %v", err)
	}
	log.Infof("Started text detection job with JobID: %s", jobID)

	// Poll for job completion and retrieve results
	blocks, err := getDocumentTextDetection(textractClient, jobID)
	if err != nil {
		return "", fmt.Errorf("failed to get text detection results: %v", err)
	}

	// Extract and return the text from the blocks
	extractedText := extractTextFromBlocks(blocks)

	// Store result in cache
	ocrCache.mutex.Lock()
	if ocrCache.cacheList.Len() >= maxCacheSize {
		evictElement := ocrCache.cacheList.Back()
		if evictElement != nil {
			ocrCache.cacheList.Remove(evictElement)
			delete(ocrCache.cacheMap, evictElement.Value.(CacheEntry).key)
		}
	}
	newEntry := CacheEntry{key: documentId, value: extractedText}
	ocrCache.cacheList.PushFront(newEntry)
	ocrCache.cacheMap[documentId] = extractedText
	ocrCache.mutex.Unlock()

	return extractedText, nil
}

// deleteFromS3 deletes a file from a specified S3 bucket.
func deleteFromS3(client *s3.Client, bucketName, objectKey string) error {
	_, err := client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	return err
}

// uploadToS3 uploads a byte array to a specified S3 bucket.
func uploadToS3(client *s3.Client, bucketName, objectKey string, fileBytes []byte) error {
	_, err := client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
		Body:   bytes.NewReader(fileBytes),
	})
	return err
}

// startDocumentTextDetection starts an asynchronous text detection job on a PDF file stored in S3.
func startDocumentTextDetection(client *textract.Client, bucketName, objectKey string) (string, error) {
	input := &textract.StartDocumentTextDetectionInput{
		DocumentLocation: &types.DocumentLocation{
			S3Object: &types.S3Object{
				Bucket: aws.String(bucketName),
				Name:   aws.String(objectKey),
			},
		},
	}

	resp, err := client.StartDocumentTextDetection(context.TODO(), input)
	if err != nil {
		return "", fmt.Errorf("failed to start document text detection: %v", err)
	}
	return *resp.JobId, nil
}

// getDocumentTextDetection polls for the results of a text detection job.
func getDocumentTextDetection(client *textract.Client, jobId string) ([]types.Block, error) {
	var blocks []types.Block
	var nextToken *string

	ctx := context.Background()
	for {
		resp, err := client.GetDocumentTextDetection(ctx, &textract.GetDocumentTextDetectionInput{
			JobId:     aws.String(jobId),
			NextToken: nextToken,
		})
		if err != nil {
			log.Error("Failed to get document text detection result")
			return nil, fmt.Errorf("failed to get document text detection result: %v", err)
		}

		if resp.JobStatus == types.JobStatusSucceeded {
			blocks = append(blocks, resp.Blocks...)
			if resp.NextToken == nil {
				log.Info("OCR Job completed successfully")
				break
			} else {
				log.Info("Retrieving next set of blocks...")
				nextToken = resp.NextToken
			}
		} else if resp.JobStatus != types.JobStatusInProgress {
			log.Error("OCR Job failed")
			return nil, fmt.Errorf("document text detection job failed with status: %v", resp.JobStatus)
		} else {
			log.Info("OCR Job still in progress, waiting 3 seconds before retrying...")
			time.Sleep(3 * time.Second)
		}
	}

	return blocks, nil
}

// extractTextFromBlocks extracts text from the Textract blocks and returns it as a single string.
func extractTextFromBlocks(blocks []types.Block) string {
	var extractedText string
	for _, block := range blocks {
		if block.BlockType == types.BlockTypeLine {
			extractedText += *block.Text + "\n"
		}
	}
	return extractedText
}
