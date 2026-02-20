// SPDX-License-Identifier: Apache-2.0

package server

import (
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// resetQueueManager resets the global queue manager for test isolation
func resetQueueManager() {
	queueManager = NewQueueManager()
}

// --- CreateQueue tests ---

func TestCreateQueue_FormEncoded(t *testing.T) {
	resetQueueManager()

	form := url.Values{}
	form.Set("Action", "CreateQueue")
	form.Set("QueueName", "test-queue")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify XML response contains QueueUrl
	body := rr.Body.String()
	if !strings.Contains(body, "<QueueUrl>") {
		t.Errorf("expected XML response with QueueUrl, got: %s", body)
	}
	if !strings.Contains(body, "/test-queue") {
		t.Errorf("expected queue URL to contain /test-queue, got: %s", body)
	}
}

func TestCreateQueue_JSON(t *testing.T) {
	resetQueueManager()

	payload := `{"QueueName":"json-queue"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if !strings.Contains(resp["QueueUrl"], "/json-queue") {
		t.Errorf("expected QueueUrl to contain /json-queue, got: %s", resp["QueueUrl"])
	}
}

func TestCreateQueue_MissingName(t *testing.T) {
	resetQueueManager()

	form := url.Values{}
	form.Set("Action", "CreateQueue")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rr.Code)
	}
}

// --- ListQueues tests ---

func TestListQueues_Empty(t *testing.T) {
	resetQueueManager()

	form := url.Values{}
	form.Set("Action", "ListQueues")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}
}

func TestListQueues_WithQueues(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("queue-a", map[string]string{})
	queueManager.CreateQueue("queue-b", map[string]string{})

	form := url.Values{}
	form.Set("Action", "ListQueues")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "queue-a") || !strings.Contains(body, "queue-b") {
		t.Errorf("expected both queues in response, got: %s", body)
	}
}

func TestListQueues_JSON(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("my-queue", map[string]string{})

	payload := `{}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.ListQueues")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string][]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if len(resp["QueueUrls"]) != 1 {
		t.Errorf("expected 1 queue URL, got %d", len(resp["QueueUrls"]))
	}
}

// --- SendMessage + ReceiveMessage tests ---

func TestSendAndReceiveMessage(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("msg-queue", map[string]string{})

	router := SetupRouter()

	// Send a message
	form := url.Values{}
	form.Set("Action", "SendMessage")
	form.Set("QueueUrl", "http://localhost/msg-queue")
	form.Set("MessageBody", "hello world")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("SendMessage: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify response contains MessageId
	body := rr.Body.String()
	if !strings.Contains(body, "<MessageId>") {
		t.Errorf("expected MessageId in response, got: %s", body)
	}

	// Receive the message
	form = url.Values{}
	form.Set("Action", "ReceiveMessage")
	form.Set("QueueUrl", "http://localhost/msg-queue")
	form.Set("MaxNumberOfMessages", "1")

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("ReceiveMessage: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body = rr.Body.String()
	if !strings.Contains(body, "hello world") {
		t.Errorf("expected message body in response, got: %s", body)
	}
}

func TestSendMessage_JSON(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("json-msg-queue", map[string]string{})

	payload := `{"QueueUrl":"http://localhost/json-msg-queue","MessageBody":"json hello"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessage")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if resp["MessageId"] == "" {
		t.Error("expected non-empty MessageId")
	}
	if resp["MD5OfMessageBody"] == "" {
		t.Error("expected non-empty MD5OfMessageBody")
	}
}

func TestSendMessage_NonExistentQueue(t *testing.T) {
	resetQueueManager()

	form := url.Values{}
	form.Set("Action", "SendMessage")
	form.Set("QueueUrl", "http://localhost/no-such-queue")
	form.Set("MessageBody", "test")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- DeleteMessage tests ---

func TestDeleteMessage(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("del-queue", map[string]string{})

	router := SetupRouter()

	// Send a message
	form := url.Values{}
	form.Set("Action", "SendMessage")
	form.Set("QueueUrl", "http://localhost/del-queue")
	form.Set("MessageBody", "to be deleted")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Receive to get receipt handle
	form = url.Values{}
	form.Set("Action", "ReceiveMessage")
	form.Set("QueueUrl", "http://localhost/del-queue")

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Parse receipt handle from XML
	type ReceiveResult struct {
		Messages []struct {
			ReceiptHandle string `xml:"ReceiptHandle"`
		} `xml:"ReceiveMessageResult>Message"`
	}
	var result ReceiveResult
	if err := xml.NewDecoder(rr.Body).Decode(&result); err != nil {
		t.Fatalf("failed to parse receive response: %v", err)
	}
	if len(result.Messages) == 0 {
		t.Fatal("expected at least one message")
	}

	receiptHandle := result.Messages[0].ReceiptHandle

	// Delete the message
	form = url.Values{}
	form.Set("Action", "DeleteMessage")
	form.Set("QueueUrl", "http://localhost/del-queue")
	form.Set("ReceiptHandle", receiptHandle)

	req = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("DeleteMessage: expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

// --- DeleteQueue tests ---

func TestDeleteQueue(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("doomed-queue", map[string]string{})

	form := url.Values{}
	form.Set("Action", "DeleteQueue")
	form.Set("QueueUrl", "http://localhost/doomed-queue")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Verify queue is gone
	_, exists := queueManager.GetQueue("doomed-queue")
	if exists {
		t.Error("queue should have been deleted")
	}
}

func TestDeleteQueue_NonExistent(t *testing.T) {
	resetQueueManager()

	form := url.Values{}
	form.Set("Action", "DeleteQueue")
	form.Set("QueueUrl", "http://localhost/no-such-queue")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- GetQueueAttributes tests ---

func TestGetQueueAttributes(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("attr-queue", map[string]string{})

	form := url.Values{}
	form.Set("Action", "GetQueueAttributes")
	form.Set("QueueUrl", "http://localhost/attr-queue")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	body := rr.Body.String()
	if !strings.Contains(body, "VisibilityTimeout") {
		t.Errorf("expected VisibilityTimeout in attributes, got: %s", body)
	}
}

func TestGetQueueAttributes_JSON(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("json-attr-queue", map[string]string{})

	payload := `{"QueueUrl":"http://localhost/json-attr-queue"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.GetQueueAttributes")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	attrs := resp["Attributes"]
	if attrs["VisibilityTimeout"] != "30" {
		t.Errorf("expected VisibilityTimeout=30, got %s", attrs["VisibilityTimeout"])
	}
	if attrs["QueueArn"] == "" {
		t.Error("expected non-empty QueueArn")
	}
}

// --- PurgeQueue tests ---

func TestPurgeQueue(t *testing.T) {
	resetQueueManager()
	q, _ := queueManager.CreateQueue("purge-queue", map[string]string{})
	q.SendMessage("msg1", nil, 0, "", "")
	q.SendMessage("msg2", nil, 0, "", "")

	form := url.Values{}
	form.Set("Action", "PurgeQueue")
	form.Set("QueueUrl", "http://localhost/purge-queue")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Verify messages are purged
	q.mu.RLock()
	count := len(q.Messages)
	q.mu.RUnlock()
	if count != 0 {
		t.Errorf("expected 0 messages after purge, got %d", count)
	}
}

// --- SetQueueAttributes tests ---

func TestSetQueueAttributes(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("set-attr-queue", map[string]string{})

	payload := `{"QueueUrl":"http://localhost/set-attr-queue","Attributes":{"VisibilityTimeout":"60","DelaySeconds":"5"}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.SetQueueAttributes")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify attributes were set
	q, _ := queueManager.GetQueue("set-attr-queue")
	if q.VisibilityTimeout != 60 {
		t.Errorf("expected VisibilityTimeout=60, got %d", q.VisibilityTimeout)
	}
	if q.DelaySeconds != 5 {
		t.Errorf("expected DelaySeconds=5, got %d", q.DelaySeconds)
	}
}

// --- FIFO Queue tests ---

func TestFIFOQueue(t *testing.T) {
	resetQueueManager()

	payload := `{"QueueName":"test.fifo","Attributes":{"FifoQueue":"true","ContentBasedDeduplication":"true"}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.CreateQueue")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	q, exists := queueManager.GetQueue("test.fifo")
	if !exists {
		t.Fatal("FIFO queue should exist")
	}
	if !q.FifoQueue {
		t.Error("expected FifoQueue=true")
	}
	if !q.ContentBasedDeduplication {
		t.Error("expected ContentBasedDeduplication=true")
	}
}

// --- Health endpoint test ---

func TestHealthEndpoint(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %s", resp["status"])
	}
	if resp["service"] != "ess-queue-ess" {
		t.Errorf("expected service=ess-queue-ess, got %s", resp["service"])
	}
}

// --- Unknown action test ---

func TestUnknownAction(t *testing.T) {
	resetQueueManager()

	form := url.Values{}
	form.Set("Action", "DoSomethingWeird")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

// --- Batch operations tests ---

func TestSendMessageBatch_JSON(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("batch-queue", map[string]string{})

	payload := `{
		"QueueUrl":"http://localhost/batch-queue",
		"Entries":[
			{"Id":"msg1","MessageBody":"hello one"},
			{"Id":"msg2","MessageBody":"hello two"},
			{"Id":"msg3","MessageBody":"hello three"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.SendMessageBatch")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	successful, ok := resp["Successful"].([]interface{})
	if !ok {
		t.Fatal("expected Successful array")
	}
	if len(successful) != 3 {
		t.Errorf("expected 3 successful entries, got %d", len(successful))
	}
}

func TestDeleteMessageBatch_JSON(t *testing.T) {
	resetQueueManager()
	q, _ := queueManager.CreateQueue("batch-del-queue", map[string]string{})

	router := SetupRouter()

	// Send messages
	q.SendMessage("msg1", nil, 0, "", "")
	q.SendMessage("msg2", nil, 0, "", "")

	// Receive them to get receipt handles
	msgs := q.ReceiveMessages(2, 30, 0)
	if len(msgs) < 2 {
		t.Fatal("expected 2 messages")
	}

	payload := `{
		"QueueUrl":"http://localhost/batch-del-queue",
		"Entries":[
			{"Id":"del1","ReceiptHandle":"` + msgs[0].ReceiptHandle + `"},
			{"Id":"del2","ReceiptHandle":"` + msgs[1].ReceiptHandle + `"}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.DeleteMessageBatch")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	successful, ok := resp["Successful"].([]interface{})
	if !ok {
		t.Fatal("expected Successful array")
	}
	if len(successful) != 2 {
		t.Errorf("expected 2 successful deletions, got %d", len(successful))
	}
}

func TestChangeMessageVisibility_JSON(t *testing.T) {
	resetQueueManager()
	q, _ := queueManager.CreateQueue("vis-queue", map[string]string{})
	q.SendMessage("test message", nil, 0, "", "")

	msgs := q.ReceiveMessages(1, 30, 0)
	if len(msgs) == 0 {
		t.Fatal("expected a message")
	}

	payload := `{
		"QueueUrl":"http://localhost/vis-queue",
		"ReceiptHandle":"` + msgs[0].ReceiptHandle + `",
		"VisibilityTimeout":60
	}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.ChangeMessageVisibility")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestGetQueueUrl_JSON(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("url-queue", map[string]string{})

	payload := `{"QueueName":"url-queue"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.GetQueueUrl")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if !strings.Contains(resp["QueueUrl"], "/url-queue") {
		t.Errorf("expected QueueUrl to contain /url-queue, got: %s", resp["QueueUrl"])
	}
}

func TestGetQueueUrl_NonExistent(t *testing.T) {
	resetQueueManager()

	payload := `{"QueueName":"no-such-queue"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	req.Header.Set("X-Amz-Target", "AmazonSQS.GetQueueUrl")
	req.Header.Set("Content-Type", "application/x-amz-json-1.0")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestMessageAttributes_FormEncoded(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("attr-msg-queue", map[string]string{})

	form := url.Values{}
	form.Set("Action", "SendMessage")
	form.Set("QueueUrl", "http://localhost/attr-msg-queue")
	form.Set("MessageBody", "test body")
	form.Set("MessageAttribute.1.Name", "CustomAttr")
	form.Set("MessageAttribute.1.Value.DataType", "String")
	form.Set("MessageAttribute.1.Value.StringValue", "my-value")

	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	// Verify the message was stored with attributes
	q, _ := queueManager.GetQueue("attr-msg-queue")
	q.mu.RLock()
	defer q.mu.RUnlock()
	if len(q.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(q.Messages))
	}
	msg := q.Messages[0]
	if len(msg.MessageAttributes) == 0 {
		t.Error("expected message attributes to be set")
	}
	attr, ok := msg.MessageAttributes["CustomAttr"]
	if !ok {
		t.Error("expected CustomAttr in message attributes")
	}
	attrMap, ok := attr.(map[string]interface{})
	if !ok {
		t.Fatalf("expected attribute to be a map, got %T", attr)
	}
	if attrMap["StringValue"] != "my-value" {
		t.Errorf("expected StringValue=my-value, got %v", attrMap["StringValue"])
	}
}

// --- Admin API tests ---

func TestAdminAPI_Queues(t *testing.T) {
	resetQueueManager()
	queueManager.CreateQueue("admin-queue", map[string]string{})

	req := httptest.NewRequest(http.MethodGet, "/admin/api/queues", nil)
	rr := httptest.NewRecorder()

	router := SetupRouter()
	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode admin API response: %v", err)
	}
	queues, ok := resp["queues"].([]interface{})
	if !ok {
		t.Fatal("expected 'queues' array in response")
	}
	if len(queues) != 1 {
		t.Errorf("expected 1 queue, got %d", len(queues))
	}
}
