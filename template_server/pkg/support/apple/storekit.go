// storekit.go StoreKit 2 App Store Server API 客户端
package apple

import (
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

//go:embed apple_root_ca_g3.pem
var appleRootCAG3PEM []byte

// StoreKitConfig StoreKit 2 / App Store Server API 所需配置
// 所有字段均用于 JWT 认证与 API 调用，需在 App Store Connect 创建 In-App Purchase API Key 后获取
type StoreKitConfig struct {
	// KeyID App Store Connect → 用户和访问 → 密钥 → In-App Purchase 的 Key ID，JWT 头 kid
	KeyID string `json:"key_id"`
	// IssuerID App Store Connect → 用户和访问 → 密钥 页面的 Issuer ID（UUID），JWT 声明 iss
	IssuerID string `json:"issuer_id"`
	// BundleID 应用 Bundle ID，与 Xcode 一致，JWT 声明 bid，用于校验交易归属
	BundleID string `json:"bundle_id"`
	// PrivateKey 上述 Key 下载的 .p8 文件内容（PKCS#8 ECDSA），用于签发 JWT，ES256
	PrivateKey string `json:"private_key"`
	// Sandbox true 使用沙箱 API（api.storekit-sandbox.itunes.apple.com），false 使用生产环境
	Sandbox bool `json:"sandbox"`
}

// StoreKitClient StoreKit 客户端
type StoreKitClient struct {
	config     *StoreKitConfig
	privateKey *ecdsa.PrivateKey
	httpClient *http.Client
	baseURL    string
}

// NewStoreKitClient 创建 StoreKit 客户端
func NewStoreKitClient(config *StoreKitConfig) (*StoreKitClient, error) {
	// 解析私钥
	privateKey, err := parsePrivateKey(config.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// 确定基础URL
	baseURL := "https://api.storekit.itunes.apple.com"
	if config.Sandbox {
		baseURL = "https://api.storekit-sandbox.itunes.apple.com"
	}

	return &StoreKitClient{
		config:     config,
		privateKey: privateKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    baseURL,
	}, nil
}

// parsePrivateKey 解析 PEM 格式的私钥（.p8 内容）
// 支持：① 标准 PEM 多行；② YAML 单行里用 \n 表示换行；③ 仅 base64 内容（无头尾）
func parsePrivateKey(keyData string) (*ecdsa.PrivateKey, error) {
	keyData = strings.TrimSpace(keyData)
	if keyData == "" {
		return nil, errors.New("private key is empty")
	}
	// YAML 中若写成单行，可把字面量 \n 换成真实换行
	keyData = strings.ReplaceAll(keyData, "\\n", "\n")

	// 先按完整 PEM 解析（含 -----BEGIN/END PRIVATE KEY-----）
	block, _ := pem.Decode([]byte(keyData))
	if block == nil {
		// 无 PEM 头尾时当作纯 base64：去掉空白后解码
		b64 := strings.ReplaceAll(strings.ReplaceAll(keyData, "\n", ""), " ", "")
		raw, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return nil, fmt.Errorf("private key not valid PEM or base64: %w", err)
		}
		block = &pem.Block{Type: "PRIVATE KEY", Bytes: raw}
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not ECDSA format")
	}
	return ecdsaKey, nil
}

// generateJWT 生成 JWT Token 用于 App Store Server API 认证
func (c *StoreKitClient) generateJWT() (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"iss": c.config.IssuerID,
		"iat": now.Unix(),
		"exp": now.Add(20 * time.Minute).Unix(),
		"aud": "appstoreconnect-v1",
		"bid": c.config.BundleID,
	})

	token.Header["kid"] = c.config.KeyID

	return token.SignedString(c.privateKey)
}

// TransactionInfo 交易信息
type TransactionInfo struct {
	TransactionID         string  `json:"transactionId"`
	OriginalTransactionID string  `json:"originalTransactionId"`
	ProductID             string  `json:"productId"`
	BundleID              string  `json:"bundleId"`
	PurchaseDate          int64   `json:"purchaseDate"`
	ExpiresDate           *int64  `json:"expiresDate"`
	Quantity              int     `json:"quantity"`
	Type                  string  `json:"type"`
	Environment           string  `json:"environment"`
	SignedDate            int64   `json:"signedDate"`
	RevocationDate        *int64  `json:"revocationDate"`
	RevocationReason      *int    `json:"revocationReason"`
	IsUpgraded            bool    `json:"isUpgraded"`
	OfferType             *int    `json:"offerType"`
	OfferIdentifier       *string `json:"offerIdentifier"`
	WebOrderLineItemID    *string `json:"webOrderLineItemId"`
	SubscriptionGroupID   *string `json:"subscriptionGroupIdentifier"`
	InAppOwnershipType    string  `json:"inAppOwnershipType"`
}

// TransactionResponse 交易响应
type TransactionResponse struct {
	SignedTransactionInfo string `json:"signedTransactionInfo"`
}

// GetTransactionInfo 获取交易信息
func (c *StoreKitClient) GetTransactionInfo(ctx context.Context, transactionID string) (*TransactionInfo, error) {
	jwtToken, err := c.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %w", err)
	}

	url := fmt.Sprintf("%s/inApps/v1/transactions/%s", c.baseURL, transactionID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var transactionResp TransactionResponse
	if err := json.NewDecoder(resp.Body).Decode(&transactionResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// 解析 JWT 交易信息
	transactionInfo, err := c.parseTransactionJWT(transactionResp.SignedTransactionInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transaction JWT: %w", err)
	}

	return transactionInfo, nil
}

// verifyJWSWithX5C 使用 x5c 证书链验证 JWS 签名（StoreKit 2 规范）
func (c *StoreKitClient) verifyJWSWithX5C(tokenString string) (*jwt.Token, jwt.MapClaims, error) {
	token, err := new(jwt.Parser).Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodES256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		x5cVal, ok := token.Header["x5c"]
		if !ok {
			return nil, errors.New("JWS header missing x5c")
		}
		x5c, ok := x5cVal.([]interface{})
		if !ok || len(x5c) == 0 {
			return nil, errors.New("invalid x5c format")
		}
		leafEnc, ok := x5c[0].(string)
		if !ok {
			return nil, errors.New("invalid x5c[0]")
		}
		leafDER, err := base64.StdEncoding.DecodeString(leafEnc)
		if err != nil {
			return nil, fmt.Errorf("decode x5c leaf: %w", err)
		}
		leafCert, err := x509.ParseCertificate(leafDER)
		if err != nil {
			return nil, fmt.Errorf("parse leaf cert: %w", err)
		}
		var intermediates []*x509.Certificate
		for i := 1; i < len(x5c); i++ {
			enc, ok := x5c[i].(string)
			if !ok {
				continue
			}
			der, err := base64.StdEncoding.DecodeString(enc)
			if err != nil {
				continue
			}
			cert, err := x509.ParseCertificate(der)
			if err != nil {
				continue
			}
			intermediates = append(intermediates, cert)
		}
		block, _ := pem.Decode(appleRootCAG3PEM)
		if block == nil {
			return nil, errors.New("failed to decode Apple Root CA G3 PEM")
		}
		rootCert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse Apple Root CA: %w", err)
		}
		roots := x509.NewCertPool()
		roots.AddCert(rootCert)
		interPool := x509.NewCertPool()
		for _, ic := range intermediates {
			interPool.AddCert(ic)
		}
		_, err = leafCert.Verify(x509.VerifyOptions{Roots: roots, Intermediates: interPool})
		if err != nil {
			return nil, fmt.Errorf("certificate chain verification failed: %w", err)
		}
		pub, ok := leafCert.PublicKey.(*ecdsa.PublicKey)
		if !ok {
			return nil, errors.New("leaf cert is not ECDSA")
		}
		return pub, nil
	})
	if err != nil {
		return nil, nil, err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, nil, errors.New("invalid token or claims")
	}
	return token, claims, nil
}

// NotificationPayloadV2 App Store Server Notifications V2 解码后的 payload
type NotificationPayloadV2 struct {
	NotificationType      string // SUBSCRIBED, DID_RENEW, EXPIRED, REFUND 等
	Subtype               string // INITIAL_BUY, RESUBSCRIBE 等（可选）
	SignedTransactionInfo string // 嵌套的 JWT，可解码得到 originalTransactionId
	SignedRenewalInfo     string // 续订信息 JWT（可选）
}

// ParseNotificationPayload 解析并验证 V2 服务器通知的 signedPayload（StoreKit 2 规范：必须验证 JWS 签名）
func (c *StoreKitClient) ParseNotificationPayload(signedPayload string) (*NotificationPayloadV2, error) {
	if signedPayload == "" {
		return nil, errors.New("signedPayload is empty")
	}
	_, claims, err := c.verifyJWSWithX5C(signedPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to verify notification JWS: %w", err)
	}
	out := &NotificationPayloadV2{}
	if v, ok := claims["notificationType"].(string); ok {
		out.NotificationType = v
	}
	if v, ok := claims["subtype"].(string); ok {
		out.Subtype = v
	}
	data, ok := claims["data"].(map[string]interface{})
	if !ok || data == nil {
		return nil, errors.New("notification payload missing data")
	}
	if v, ok := data["signedTransactionInfo"].(string); ok {
		out.SignedTransactionInfo = v
	}
	if v, ok := data["signedRenewalInfo"].(string); ok {
		out.SignedRenewalInfo = v
	}
	return out, nil
}

// DecodeTransactionInfo 从 signedTransactionInfo JWT 解码并验证交易信息（StoreKit 2 规范：必须验证 JWS 签名）
func (c *StoreKitClient) DecodeTransactionInfo(signedTransactionInfo string) (*TransactionInfo, error) {
	return c.parseTransactionJWT(signedTransactionInfo)
}

// parseTransactionJWT 解析并验证交易 JWT（使用 x5c 证书链验证 Apple 签名）
func (c *StoreKitClient) parseTransactionJWT(signedTransactionInfo string) (*TransactionInfo, error) {
	if signedTransactionInfo == "" {
		return nil, errors.New("signedTransactionInfo is empty")
	}
	_, claims, err := c.verifyJWSWithX5C(signedTransactionInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to verify transaction JWS: %w", err)
	}

	transactionInfo := &TransactionInfo{}

	// 解析字段
	if val, ok := claims["transactionId"].(string); ok {
		transactionInfo.TransactionID = val
	}
	if val, ok := claims["originalTransactionId"].(string); ok {
		transactionInfo.OriginalTransactionID = val
	}
	if val, ok := claims["productId"].(string); ok {
		transactionInfo.ProductID = val
	}
	if val, ok := claims["bundleId"].(string); ok {
		transactionInfo.BundleID = val
	}
	if val, ok := claims["purchaseDate"].(float64); ok {
		transactionInfo.PurchaseDate = int64(val)
	}
	if val, ok := claims["expiresDate"].(float64); ok {
		expiresDate := int64(val)
		transactionInfo.ExpiresDate = &expiresDate
	}
	if val, ok := claims["quantity"].(float64); ok {
		transactionInfo.Quantity = int(val)
	}
	if val, ok := claims["type"].(string); ok {
		transactionInfo.Type = val
	}
	if val, ok := claims["environment"].(string); ok {
		transactionInfo.Environment = val
	}
	if val, ok := claims["signedDate"].(float64); ok {
		transactionInfo.SignedDate = int64(val)
	}
	if val, ok := claims["revocationDate"].(float64); ok {
		revocationDate := int64(val)
		transactionInfo.RevocationDate = &revocationDate
	}
	if val, ok := claims["revocationReason"].(float64); ok {
		reason := int(val)
		transactionInfo.RevocationReason = &reason
	}
	if val, ok := claims["isUpgraded"].(bool); ok {
		transactionInfo.IsUpgraded = val
	}
	if val, ok := claims["offerType"].(float64); ok {
		offerType := int(val)
		transactionInfo.OfferType = &offerType
	}
	if val, ok := claims["offerIdentifier"].(string); ok {
		transactionInfo.OfferIdentifier = &val
	}
	if val, ok := claims["webOrderLineItemId"].(string); ok {
		transactionInfo.WebOrderLineItemID = &val
	}
	if val, ok := claims["subscriptionGroupIdentifier"].(string); ok {
		transactionInfo.SubscriptionGroupID = &val
	}
	if val, ok := claims["inAppOwnershipType"].(string); ok {
		transactionInfo.InAppOwnershipType = val
	}

	return transactionInfo, nil
}

// GetTransactionHistory 获取交易历史
func (c *StoreKitClient) GetTransactionHistory(ctx context.Context, originalTransactionID string) ([]*TransactionInfo, error) {
	jwtToken, err := c.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %w", err)
	}

	url := fmt.Sprintf("%s/inApps/v1/history/%s", c.baseURL, originalTransactionID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var historyResp struct {
		SignedTransactions []string `json:"signedTransactions"`
		HasMore            bool     `json:"hasMore"`
		Revision           string   `json:"revision"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&historyResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var transactions []*TransactionInfo
	for _, signedTransaction := range historyResp.SignedTransactions {
		transactionInfo, err := c.parseTransactionJWT(signedTransaction)
		if err != nil {
			return nil, fmt.Errorf("failed to parse transaction: %w", err)
		}
		transactions = append(transactions, transactionInfo)
	}

	return transactions, nil
}

// StatusResponse 状态响应
type StatusResponse struct {
	Data []struct {
		Type       string `json:"type"`
		ID         string `json:"id"`
		Attributes struct {
			Name                  string `json:"name"`
			ProductID             string `json:"productId"`
			Status                int    `json:"status"`
			SubscriptionStatusURL string `json:"subscriptionStatusUrl"`
			Environment           string `json:"environment"`
			LastTransactions      []struct {
				OriginalTransactionID string `json:"originalTransactionId"`
				Status                int    `json:"status"`
				SignedRenewalInfo     string `json:"signedRenewalInfo"`
				SignedTransactionInfo string `json:"signedTransactionInfo"`
			} `json:"lastTransactions"`
		} `json:"attributes"`
	} `json:"data"`
}

// GetSubscriptionStatus 获取订阅状态
func (c *StoreKitClient) GetSubscriptionStatus(ctx context.Context, originalTransactionID string) (*StatusResponse, error) {
	jwtToken, err := c.generateJWT()
	if err != nil {
		return nil, fmt.Errorf("failed to generate JWT: %w", err)
	}

	url := fmt.Sprintf("%s/inApps/v1/subscriptions/%s", c.baseURL, originalTransactionID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d - %s", resp.StatusCode, string(body))
	}

	var statusResp StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &statusResp, nil
}
