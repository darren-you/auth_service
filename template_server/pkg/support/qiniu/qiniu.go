// Copyright 2025 Stellar Development Team
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// qiniu.go 七牛云客户端封装
package qiniu

import (
	"bytes"
	"context"

	"github.com/qiniu/api.v7/v7/auth/qbox"
	"github.com/qiniu/api.v7/v7/storage"
)

// Client 七牛云客户端
type Client struct {
	mac       *qbox.Mac
	bucket    string
	domain    string
	putPolicy storage.PutPolicy
}

// NewClient 创建七牛云客户端实例
func NewClient(accessKey, secretKey, bucket, domain string) *Client {
	mac := qbox.NewMac(accessKey, secretKey)
	putPolicy := storage.PutPolicy{Scope: bucket}
	return &Client{
		mac:       mac,
		bucket:    bucket,
		domain:    domain,
		putPolicy: putPolicy,
	}
}

// UploadFile 上传本地文件到七牛云
// filePath: 本地文件路径
// key: 七牛云存储的文件路径（如果为空则自动生成）
// return: 文件URL和错误信息
func (c *Client) UploadFile(filePath string, key string) (string, error) {
	upToken := c.putPolicy.UploadToken(c.mac)
	cfg := storage.Config{Zone: &storage.Zone_z2}
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}
	err := formUploader.PutFile(nil, &ret, upToken, key, filePath, nil)
	if err != nil {
		return "", err
	}
	return c.domain + "/" + ret.Key, nil
}

// UploadData 上传字节数据到七牛云
// data: 文件字节数据
// key: 七牛云存储的文件路径（如果为空则自动生成）
// return: 文件URL和错误信息
func (c *Client) UploadData(data []byte, key string) (string, error) {
	upToken := c.putPolicy.UploadToken(c.mac)
	cfg := storage.Config{Zone: &storage.Zone_z2}
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}
	err := formUploader.Put(context.Background(), &ret, upToken, key, bytes.NewReader(data), int64(len(data)), nil)
	if err != nil {
		return "", err
	}
	return c.domain + "/" + ret.Key, nil
}

// UploadStream 上传文件流到七牛云
// reader: 文件流
// size: 文件大小
// key: 七牛云存储的文件路径（如果为空则自动生成）
// return: 文件URL和错误信息
func (c *Client) UploadStream(reader *bytes.Reader, size int64, key string) (string, error) {
	upToken := c.putPolicy.UploadToken(c.mac)
	cfg := storage.Config{Zone: &storage.Zone_z2}
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}
	err := formUploader.Put(nil, &ret, upToken, key, reader, size, nil)
	if err != nil {
		return "", err
	}
	return c.domain + "/" + ret.Key, nil
}
