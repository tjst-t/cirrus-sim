//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func awxBaseURL(t *testing.T) string {
	t.Helper()
	port := os.Getenv("AWX_SIM_PORT")
	if port == "" {
		port = "8300"
	}
	return fmt.Sprintf("http://localhost:%s", port)
}

func awxReset(t *testing.T, baseURL string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/sim/reset", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("reset failed: %v", err)
	}
	resp.Body.Close()
}

func awxPost(t *testing.T, url string, body interface{}) map[string]interface{} {
	t.Helper()
	data, _ := json.Marshal(body)
	resp, err := http.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		t.Fatalf("POST %s returned %d: %s", url, resp.StatusCode, respBody)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("unmarshal response: %v, body: %s", err, respBody)
	}
	return result
}

func awxGet(t *testing.T, url string) map[string]interface{} {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s failed: %v", url, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		t.Fatalf("GET %s returned %d: %s", url, resp.StatusCode, respBody)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("unmarshal response: %v, body: %s", err, respBody)
	}
	return result
}

func TestAWXJobTemplateLifecycle(t *testing.T) {
	baseURL := awxBaseURL(t)
	awxReset(t, baseURL)

	// Create a job template
	tmpl := awxPost(t, baseURL+"/api/v2/job_templates/", map[string]interface{}{
		"name":                "deploy-vm",
		"description":         "Deploy a virtual machine",
		"expected_duration_ms": 100,
		"failure_rate":         0.0,
	})

	tmplID := tmpl["id"].(float64)
	if tmplID != 1 {
		t.Errorf("template id = %v, want 1", tmplID)
	}
	if tmpl["name"] != "deploy-vm" {
		t.Errorf("template name = %v, want deploy-vm", tmpl["name"])
	}
	t.Logf("Created template: id=%.0f name=%v", tmplID, tmpl["name"])

	// Get template by ID
	got := awxGet(t, fmt.Sprintf("%s/api/v2/job_templates/%.0f/", baseURL, tmplID))
	if got["name"] != "deploy-vm" {
		t.Errorf("get template name = %v, want deploy-vm", got["name"])
	}

	// List templates
	list := awxGet(t, baseURL+"/api/v2/job_templates/")
	if list["count"].(float64) != 1 {
		t.Errorf("template count = %v, want 1", list["count"])
	}
	results := list["results"].([]interface{})
	if len(results) != 1 {
		t.Errorf("results len = %d, want 1", len(results))
	}
}

func TestAWXJobLaunchAndComplete(t *testing.T) {
	baseURL := awxBaseURL(t)
	awxReset(t, baseURL)

	// Create template with short duration and no failure
	awxPost(t, baseURL+"/api/v2/job_templates/", map[string]interface{}{
		"name":                "quick-job",
		"expected_duration_ms": 50,
		"failure_rate":         0.0,
	})

	// Launch job
	job := awxPost(t, baseURL+"/api/v2/job_templates/1/launch/", map[string]interface{}{
		"extra_vars": map[string]interface{}{"host": "host-001"},
	})

	jobID := job["id"].(float64)
	t.Logf("Launched job: id=%.0f status=%v", jobID, job["status"])

	if job["status"] != "running" {
		t.Errorf("initial status = %v, want running", job["status"])
	}
	if job["job_template"].(float64) != 1 {
		t.Errorf("job_template = %v, want 1", job["job_template"])
	}

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	// Check job status
	completed := awxGet(t, fmt.Sprintf("%s/api/v2/jobs/%.0f/", baseURL, jobID))
	t.Logf("Completed job: status=%v", completed["status"])

	if completed["status"] != "successful" {
		t.Errorf("final status = %v, want successful", completed["status"])
	}
	if completed["finished"] == nil {
		t.Error("finished timestamp should be set")
	}

	// Verify extra_vars are preserved
	extraVars := completed["extra_vars"].(map[string]interface{})
	if extraVars["host"] != "host-001" {
		t.Errorf("extra_vars.host = %v, want host-001", extraVars["host"])
	}

	// Check stats
	stats := awxGet(t, baseURL+"/sim/stats")
	if stats["successful"].(float64) != 1 {
		t.Errorf("successful count = %v, want 1", stats["successful"])
	}
}

func TestAWXJobCancel(t *testing.T) {
	baseURL := awxBaseURL(t)
	awxReset(t, baseURL)

	// Create template with long duration so we can cancel it
	awxPost(t, baseURL+"/api/v2/job_templates/", map[string]interface{}{
		"name":                "long-job",
		"expected_duration_ms": 60000,
		"failure_rate":         0.0,
	})

	// Launch job
	job := awxPost(t, baseURL+"/api/v2/job_templates/1/launch/", nil)
	jobID := job["id"].(float64)

	if job["status"] != "running" {
		t.Errorf("initial status = %v, want running", job["status"])
	}

	// Cancel the job
	canceled := awxPost(t, fmt.Sprintf("%s/api/v2/jobs/%.0f/cancel/", baseURL, jobID), nil)
	t.Logf("Canceled job: status=%v", canceled["status"])

	if canceled["status"] != "canceled" {
		t.Errorf("canceled status = %v, want canceled", canceled["status"])
	}

	// Verify via GET
	got := awxGet(t, fmt.Sprintf("%s/api/v2/jobs/%.0f/", baseURL, jobID))
	if got["status"] != "canceled" {
		t.Errorf("get status = %v, want canceled", got["status"])
	}

	// Check stats
	stats := awxGet(t, baseURL+"/sim/stats")
	if stats["canceled"].(float64) != 1 {
		t.Errorf("canceled count = %v, want 1", stats["canceled"])
	}
}

func TestAWXJobFailure(t *testing.T) {
	baseURL := awxBaseURL(t)
	awxReset(t, baseURL)

	// Create template with 100% failure rate
	awxPost(t, baseURL+"/api/v2/job_templates/", map[string]interface{}{
		"name":                "failing-job",
		"expected_duration_ms": 50,
		"failure_rate":         1.0,
	})

	// Launch job
	job := awxPost(t, baseURL+"/api/v2/job_templates/1/launch/", nil)
	jobID := job["id"].(float64)

	// Wait for completion
	time.Sleep(200 * time.Millisecond)

	got := awxGet(t, fmt.Sprintf("%s/api/v2/jobs/%.0f/", baseURL, jobID))
	t.Logf("Failed job: status=%v", got["status"])

	if got["status"] != "failed" {
		t.Errorf("status = %v, want failed", got["status"])
	}

	stats := awxGet(t, baseURL+"/sim/stats")
	if stats["failed"].(float64) != 1 {
		t.Errorf("failed count = %v, want 1", stats["failed"])
	}
}

func TestAWXResetClearsState(t *testing.T) {
	baseURL := awxBaseURL(t)
	awxReset(t, baseURL)

	// Create template and launch job
	awxPost(t, baseURL+"/api/v2/job_templates/", map[string]interface{}{
		"name":                "test",
		"expected_duration_ms": 50,
	})
	awxPost(t, baseURL+"/api/v2/job_templates/1/launch/", nil)

	// Reset
	awxReset(t, baseURL)

	// Verify templates are cleared
	list := awxGet(t, baseURL+"/api/v2/job_templates/")
	if list["count"].(float64) != 0 {
		t.Errorf("template count after reset = %v, want 0", list["count"])
	}

	// Verify stats are zeroed
	stats := awxGet(t, baseURL+"/sim/stats")
	for _, key := range []string{"pending", "running", "successful", "failed", "canceled"} {
		if stats[key].(float64) != 0 {
			t.Errorf("stats[%s] after reset = %v, want 0", key, stats[key])
		}
	}
}
