package test

import (
	"os"
	"testing"
	"time"

	"github.com/ultipa/ultipa-go-driver/v5/sdk"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
)

func TestDownload(t *testing.T) {
	fileName := "degree_min.txt"

	file, err := os.OpenFile("./data/"+fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		t.Error(err)
	}
	defer file.Close()

	receive := func(data []byte) error {
		_, err = file.Write(data)

		if err != nil {
			return err
		}
		return nil
	}
	err = client.DownloadAlgoResultFile(fileName, nil, receive)
	if err != nil {
		t.Error(err)
	}
}

func TestDownloadAll(t *testing.T) {
	receive := func(data []byte, fileName string) error {

		file, err := os.OpenFile("./data/"+fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
		if err != nil {
			t.Error(err)
		}
		defer file.Close()

		_, err = file.Write(data)

		if err != nil {
			return err
		}

		return nil
	}
	err := client.DownloadAllAlgoResultFile("8", nil, receive)
	if err != nil {
		t.Error(err)
	}
}

// TestDownloadAlgoResultFile_Louvain 测试下载单个算法结果文件
// 使用 louvain 算法生成输出文件，验证 DownloadAlgoResultFile 能正确下载指定文件
func TestDownloadAlgoResultFile_Louvain(t *testing.T) {
	// 创建连接到指定集群的 client
	config := &configuration.UltipaConfig{
		Hosts:        []string{"192.168.1.42:61299"},
		Username:     username,
		Password:     password,
		DefaultGraph: "miniCircle",
	}

	testClient, err := sdk.NewUltipaDriver(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer testClient.Close()

	// 执行 louvain 算法，生成多个输出文件
	gql := `CALL algo.louvain.write("miniCircle_hdc_graph", {
		return_id_uuid: "id",
		phase1_loop_num: 5,
		min_modularity_increase: 0.1
	}, {
		file: {
			filename_community_id: "f1.txt",
			filename_ids: "f2.txt",
			filename_num: "f3.txt"
		}
	})`

	resp, err := testClient.Gql(gql, nil)
	if err != nil {
		t.Fatalf("Failed to execute louvain algorithm: %v", err)
	}

	if !resp.IsSuccess() {
		t.Fatalf("Louvain algorithm failed: %s", resp.Status.Message)
	}

	// 从响应中获取 jobId
	jobResp, err := http.GetJobResponseFromUqlResponse(resp)
	if err != nil {
		t.Fatalf("Failed to get job response: %v", err)
	}

	jobId := jobResp.JobId
	t.Logf("Job ID: %s", jobId)

	// 等待任务完成
	time.Sleep(5 * time.Second)

	// 查询 job 获取第一个输出文件的路径
	jobs, err := testClient.ShowJob(jobId, nil)
	if err != nil {
		t.Fatalf("Failed to show job: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("Job not found")
	}

	job := jobs[0]
	outputFile, ok := job.Result["output_file"]
	if !ok {
		t.Fatal("No output_file found in job result")
	}
	t.Logf("Output file path: %s", outputFile)

	// 下载第一个算法结果文件
	var downloadedSize int
	receive := func(data []byte) error {
		downloadedSize += len(data)
		t.Logf("Downloaded chunk size: %d bytes", len(data))
		return nil
	}

	err = testClient.DownloadAlgoResultFile(outputFile, nil, receive)
	if err != nil {
		t.Fatalf("Failed to download algo result file: %v", err)
	}

	t.Logf("Successfully downloaded file, total size: %d bytes", downloadedSize)
}

// TestDownloadAllAlgoResultFile 测试下载多个算法结果文件
// 使用 louvain 算法生成多个输出文件，验证 DownloadAllAlgoResultFile 能正确下载所有文件
func TestDownloadAllAlgoResultFile(t *testing.T) {
	// 创建连接到指定集群的 client
	config := &configuration.UltipaConfig{
		Hosts:        []string{"192.168.1.42:61299"},
		Username:     username,
		Password:     password,
		DefaultGraph: "miniCircle",
	}

	testClient, err := sdk.NewUltipaDriver(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer testClient.Close()

	// 执行 louvain 算法，生成多个输出文件
	gql := `CALL algo.louvain.write("miniCircle_hdc_graph", {
		return_id_uuid: "id",
		phase1_loop_num: 5,
		min_modularity_increase: 0.1
	}, {
		file: {
			filename_community_id: "f1.txt",
			filename_ids: "f2.txt",
			filename_num: "f3.txt"
		}
	})`

	resp, err := testClient.Gql(gql, nil)
	if err != nil {
		t.Fatalf("Failed to execute louvain algorithm: %v", err)
	}

	if !resp.IsSuccess() {
		t.Fatalf("Louvain algorithm failed: %s", resp.Status.Message)
	}

	// 从响应中获取 jobId
	jobResp, err := http.GetJobResponseFromUqlResponse(resp)
	if err != nil {
		t.Fatalf("Failed to get job response: %v", err)
	}

	jobId := jobResp.JobId
	t.Logf("Job ID: %s", jobId)

	// 等待任务完成
	time.Sleep(5 * time.Second)

	// 记录下载的文件
	downloadedFiles := make(map[string]bool)

	receive := func(data []byte, fileName string) error {
		downloadedFiles[fileName] = true
		t.Logf("Downloaded chunk for file: %s, size: %d bytes", fileName, len(data))
		return nil
	}

	// 下载所有算法结果文件
	err = testClient.DownloadAllAlgoResultFile(jobId, nil, receive)
	if err != nil {
		t.Fatalf("Failed to download all algo result files: %v", err)
	}

	// 验证下载了预期的文件
	expectedFiles := []string{"f1.txt", "f2.txt", "f3.txt"}
	for _, expectedFile := range expectedFiles {
		if !downloadedFiles[expectedFile] {
			t.Errorf("Expected file %s was not downloaded", expectedFile)
		}
	}

	t.Logf("Successfully downloaded %d files", len(downloadedFiles))
}
