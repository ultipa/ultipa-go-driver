package api

import (
	"bufio"
	"errors"
	"io"
	"os"
	"path"
	"strings"

	"github.com/codingsince1985/checksum"
	ultipa "github.com/ultipa/ultipa-go-driver/v5/rpc"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/configuration"
	"github.com/ultipa/ultipa-go-driver/v5/sdk/http"
)

// UninstallHDCAlgo uninstall algo
func (api *UltipaAPI) UninstallHDCAlgo(algoName, hdcServerName string, config *configuration.RequestConfig) (*http.Response, error) {

	client, err := api.GetControlClient(config)
	if err != nil {
		return nil, err
	}

	ctx, cancel, err := api.Pool.NewContext(config)
	if err != nil {
		return nil, err
	}
	defer cancel()

	reply, err := client.UninstallAlgo(ctx, &ultipa.UninstallAlgoRequest{
		AlgoName:   algoName,
		WithServer: &ultipa.WithServer{HdcServerName: hdcServerName},
	})

	if err != nil {
		return nil, err
	}

	if reply.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
		return nil, errors.New(reply.Status.Msg)
	}

	return &http.Response{Status: &http.Status{
		Message: reply.Status.Msg,
		Code:    reply.Status.ErrorCode,
	}}, nil
}

// RollbackHDCAlgo Rollback HDC Algo
func (api *UltipaAPI) RollbackHDCAlgo(algoName, hdcServerName string, config *configuration.RequestConfig) (*http.Response, error) {

	client, err := api.GetControlClient(config)
	if err != nil {
		return nil, err
	}

	ctx, cancel, err := api.Pool.NewContext(config)
	if err != nil {
		return nil, err
	}
	defer cancel()

	reply, err := client.RollbackAlgo(ctx, &ultipa.RollbackAlgoRequest{
		AlgoName:   algoName,
		WithServer: &ultipa.WithServer{HdcServerName: hdcServerName},
	})

	if err != nil {
		return nil, err
	}

	//if reply.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
	//	return nil, errors.New(reply.Status.Msg)
	//}

	return &http.Response{Status: &http.Status{
		Message: reply.Status.Msg,
		Code:    reply.Status.ErrorCode,
	}}, nil
}

// InstallHDCAlgo  install algos, files : [...so, yml(optional)]
func (api *UltipaAPI) InstallHDCAlgo(files []string, hdcServerName string, config *configuration.RequestConfig) (*http.Response, error) {
	if files == nil || len(files) == 0 {
		return nil, errors.New("empty files")
	}
	// Validate at least one .so file exists
	hasSoFile := false
	for _, file := range files {
		if strings.HasSuffix(strings.ToLower(file), ".so") {
			hasSoFile = true
			break
		}
	}
	if !hasSoFile {
		return nil, errors.New("at least one .so file is required")
	}

	client, err := api.GetControlClient(config)
	if err != nil {
		return nil, err
	}

	ctx, cancel, err := api.Pool.NewContext(config)
	if err != nil {
		return nil, err
	}
	defer cancel()

	streamClient, err := client.InstallAlgo(ctx)
	if err != nil {
		return nil, err
	}

	// Send each so/yml file
	for _, file := range files {
		algoFile, err := os.Open(file)
		if err != nil {
			return nil, err
		}
		defer algoFile.Close()

		algoFileReader := bufio.NewReader(algoFile)
		algoFileMD5, _ := checksum.MD5sum(file)

		// Determine if file is yml by extension instead of position
		lowerFile := strings.ToLower(file)
		isYml := strings.HasSuffix(lowerFile, ".yml") || strings.HasSuffix(lowerFile, ".yaml")

		// Send  file in chunks
		if err := sendFileChunks(algoFileReader, streamClient, path.Base(algoFile.Name()), algoFileMD5, hdcServerName, isYml); err != nil {
			return nil, err
		}
	}

	reply, err := streamClient.CloseAndRecv()
	if err != nil {
		return nil, err
	}

	// Reply status check, handle empty reply
	if reply.Status == nil {
		return nil, nil
	}

	//if reply.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
	//	return nil, errors.New(reply.Status.Msg)
	//}

	return &http.Response{Status: &http.Status{
		Message: reply.Status.Msg,
		Code:    reply.Status.ErrorCode,
	}}, nil
}

// sendFileChunks sends the file in chunks to the server
func sendFileChunks(fileReader *bufio.Reader, streamClient ultipa.UltipaControls_InstallAlgoClient, fileName, md5, hdcName string, isYml bool) error {
	chunkSize := 1024 * 1024 * 1 // 1MB

	for {
		chunk := make([]byte, chunkSize)
		n, err := fileReader.Read(chunk)

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		request := &ultipa.InstallAlgoRequest{
			FileName: fileName,
			Md5:      md5,
			Chunk:    chunk[:n],
		}

		// Only include server name if provided
		if !isYml && hdcName != "" {
			request.WithServer = &ultipa.WithServer{HdcServerName: hdcName}
		}

		if err := streamClient.Send(request); err != nil {
			return err
		}
	}
	return nil
}
