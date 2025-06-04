/**
 * @Author: zhaohaichao
 * @Description:
 * @File:  exta
 * @Date: 2022/8/5 7:35 pm
 */

package api

//
//import (
//    "bufio"
//    "errors"
//    "fmt"
//    "io"
//    "os"
//    "path"
//
//    "github.com/codingsince1985/checksum"
//    ultipa " github.com/ultipa/ultipa-go-driver/rpc"
//    " github.com/ultipa/ultipa-go-driver/sdk/configuration"
//    " github.com/ultipa/ultipa-go-driver/sdk/http"
//    " github.com/ultipa/ultipa-go-driver/sdk/structs"
//)
//
//func (api *UltipaAPI) InstallExta(soFilePath string, infoFilePath string, config *configuration.RequestConfig) (*ultipa.InstallExtaReply, error) {
//
//    chunkSize := 1024 * 1024 * 1 // 2MB
//
//    // check file status
//
//    extaFile, err := os.OpenFile(soFilePath, os.O_RDONLY, 0644)
//
//    if err != nil {
//        return nil, err
//    }
//
//    extaFileReader := bufio.NewReader(extaFile)
//
//    extaFileMD5, _ := checksum.MD5sum(soFilePath)
//
//    extaInfoFile, err := os.OpenFile(infoFilePath, os.O_RDONLY, 0644)
//
//    if err != nil {
//        return nil, err
//    }
//
//    extaInfoFileReader := bufio.NewReader(extaInfoFile)
//
//    extaInfoFileMD5, _ := checksum.MD5sum(infoFilePath)
//
//    client, err := api.GetControlClient(requestConfig)
//
//    if err != nil {
//        return nil, err
//    }
//
//    ctx, cancel, err := api.Pool.NewContext(requestConfig)
//    if err != nil {
//        return nil, err
//    }
//    defer cancel()
//
//    streamClient, err := client.InstallExta(ctx)
//
//    if err != nil {
//        return nil, err
//    }
//
//    // send exta so file
//
//    for {
//
//        chunk := make([]byte, chunkSize)
//        n, err := extaFileReader.Read(chunk)
//
//        if err == io.EOF {
//            break
//        }
//
//        if err != nil {
//            return nil, err
//        }
//
//        err = streamClient.Send(&ultipa.InstallExtaRequest{
//            FileName: path.Base(extaFile.Name()),
//            Md5:      extaFileMD5,
//            Chunk:    chunk[:n],
//        })
//
//        if err != nil {
//            return nil, err
//        }
//    }
//
//    // send exta info file
//    for {
//        chunk := make([]byte, chunkSize)
//
//        n, err := extaInfoFileReader.Read(chunk)
//
//        if err == io.EOF {
//            break
//        }
//
//        if err != nil {
//            return nil, err
//        }
//
//        err = streamClient.Send(&ultipa.InstallExtaRequest{
//            FileName: path.Base(extaInfoFile.Name()),
//            Md5:      extaInfoFileMD5,
//            Chunk:    chunk[:n],
//        })
//
//        if err != nil {
//            return nil, err
//        }
//    }
//
//    reply, err := streamClient.CloseAndRecv()
//
//    if err != nil {
//        return nil, err
//    }
//
//    if reply.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
//        return nil, errors.New(reply.Status.Msg)
//    }
//
//    return reply, nil
//
//}
//
//func (api *UltipaAPI) UninstallExta(extaName string, config *configuration.RequestConfig) (*ultipa.UninstallExtaReply, error) {
//
//    client, err := api.GetControlClient(requestConfig)
//
//    if err != nil {
//        return nil, err
//    }
//
//    ctx, cancel, err := api.Pool.NewContext(requestConfig)
//    if err != nil {
//        return nil, err
//    }
//    defer cancel()
//
//    reply, err := client.UninstallExta(ctx, &ultipa.UninstallExtaRequest{
//        ExtaName: extaName,
//    })
//
//    if err != nil {
//        return nil, err
//    }
//
//    if reply.Status.ErrorCode != ultipa.ErrorCode_SUCCESS {
//        return nil, errors.New(reply.Status.Msg)
//    }
//
//    return reply, nil
//}
//
//func (api *UltipaAPI) ShowExta(config *configuration.RequestConfig) ([]*structs.Exta, error) {
//    var resp *http.UQLResponse
//    var err error
//    var extas []*structs.Exta
//
//    resp, err = api.Uql(fmt.Sprintf(`show().exta()`), config)
//    if err != nil {
//        return nil, err
//    }
//
//    extas, err = resp.Alias(http.RESP_EXTAS_KEY).AsExtas()
//
//    if len(extas) == 0 {
//        return nil, err
//    }
//
//    return extas, err
//}
