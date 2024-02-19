//nolint:tagliatelle // json naming
package xrpl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"
	rippledata "github.com/rubblelabs/ripple/data"

	"github.com/CoreumFoundation/coreum-tools/pkg/retry"
)

type rpcError struct {
	Name      string `json:"error"`
	Code      int    `json:"error_code"`
	Message   string `json:"error_message"`
	Exception string `json:"error_exception"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("failed to call RPC, error:%s, error code:%d, error message:%s, error exception:%s",
		e.Name, e.Code, e.Message, e.Exception)
}

type submitRequest struct {
	TxBlob string `json:"tx_blob"`
}

type submitResult struct {
	EngineResult        rippledata.TransactionResult `json:"engine_result"`
	EngineResultCode    int                          `json:"engine_result_code"`
	EngineResultMessage string                       `json:"engine_result_message"`
	TxBlob              string                       `json:"tx_blob"`
	Tx                  any                          `json:"tx_json"`
}

type txRequest struct {
	Transaction rippledata.Hash256 `json:"transaction"`
}

type txResult struct {
	Validated bool `json:"validated"`
	rippledata.TransactionWithMetaData
}

// UnmarshalJSON is a shim to populate the Validated field before passing control on to
// TransactionWithMetaData.UnmarshalJSON.
func (txr *txResult) UnmarshalJSON(b []byte) error {
	var extract map[string]any
	if err := json.Unmarshal(b, &extract); err != nil {
		return errors.Errorf("faild to Unmarshal to map[string]any")
	}
	validated, ok := extract["validated"]
	if ok {
		validatedVal, ok := validated.(bool)
		if !ok {
			return errors.Errorf("faild to decode object, the validated attribute is not boolean")
		}
		txr.Validated = validatedVal
	}

	return json.Unmarshal(b, &txr.TransactionWithMetaData)
}

type rpcRequest struct {
	Method string `json:"method"`
	Params []any  `json:"params,omitempty"`
}

type rpcResponse struct {
	Result any `json:"result"`
}

// AccountInfoRequest is `account_info` method request.
type AccountInfoRequest struct {
	Account     rippledata.Account `json:"account"`
	SignerLists bool               `json:"signer_lists"`
}

// AccountInfoResult is `account_info` method result.
type AccountInfoResult struct {
	LedgerSequence uint32                 `json:"ledger_current_index"`
	AccountData    AccountDataWithSigners `json:"account_data"`
}

// AccountDataWithSigners is account data with the signers list.
type AccountDataWithSigners struct {
	rippledata.AccountRoot
	SignerList []rippledata.SignerList `json:"signer_lists"`
}

// RPCClient implement the XRPL RPC client.
type RPCClient struct {
	rpcURL string
}

// NewRPCClient returns new instance of the RPCClient.
func NewRPCClient(rpcURL string) *RPCClient {
	return &RPCClient{
		rpcURL: rpcURL,
	}
}

// AccountInfo returns the account information for the given account.
func (c *RPCClient) AccountInfo(ctx context.Context, acc rippledata.Account) (AccountInfoResult, error) {
	params := AccountInfoRequest{
		Account:     acc,
		SignerLists: true,
	}
	var result AccountInfoResult
	if err := c.callRPC(ctx, "account_info", params, &result); err != nil {
		return AccountInfoResult{}, err
	}

	return result, nil
}

// SubmitAndAwaitSuccess submits tx a waits for its result, if result is not success returns an error.
func (c *RPCClient) SubmitAndAwaitSuccess(ctx context.Context, tx rippledata.Transaction) error {
	// submit the transaction
	res, err := c.submit(ctx, tx)
	if err != nil {
		return err
	}
	if !res.EngineResult.Success() {
		return errors.Errorf("the tx submition is failed, %+v", res)
	}

	retryCtx, retryCtxCancel := context.WithTimeout(ctx, time.Minute)
	defer retryCtxCancel()
	return retry.Do(retryCtx, 250*time.Millisecond, func() error {
		reqCtx, reqCtxCancel := context.WithTimeout(ctx, 3*time.Second)
		defer reqCtxCancel()
		txRes, err := c.tx(reqCtx, *tx.GetHash())
		if err != nil {
			return retry.Retryable(err)
		}
		if !txRes.Validated {
			return retry.Retryable(errors.Errorf("transaction is not validated"))
		}
		return nil
	})
}

func (c *RPCClient) submit(ctx context.Context, tx rippledata.Transaction) (submitResult, error) {
	_, raw, err := rippledata.Raw(tx)
	if err != nil {
		return submitResult{}, errors.Wrapf(err, "failed to convert transaction to raw data")
	}
	params := submitRequest{
		TxBlob: fmt.Sprintf("%X", raw),
	}
	var result submitResult
	if err := c.callRPC(ctx, "submit", params, &result); err != nil {
		return submitResult{}, err
	}

	return result, nil
}

func (c *RPCClient) tx(ctx context.Context, hash rippledata.Hash256) (txResult, error) {
	params := txRequest{
		Transaction: hash,
	}
	var result txResult
	if err := c.callRPC(ctx, "tx", params, &result); err != nil {
		return txResult{}, err
	}

	return result, nil
}

func (c *RPCClient) callRPC(ctx context.Context, method string, params, result any) error {
	request := rpcRequest{
		Method: method,
		Params: []any{
			params,
		},
	}
	err := c.doJSON(ctx, http.MethodPost, request, func(resBytes []byte) error {
		errResponse := rpcResponse{
			Result: &rpcError{},
		}
		if err := json.Unmarshal(resBytes, &errResponse); err != nil {
			return errors.Wrapf(err, "failed to decode http result to error result, raw http result:%s", string(resBytes))
		}
		errResult, ok := errResponse.Result.(*rpcError)
		if !ok {
			panic("failed to cast result to RPCError")
		}
		if errResult.Code != 0 || strings.TrimSpace(errResult.Name) != "" {
			return errors.WithStack(errResult)
		}
		response := rpcResponse{
			Result: result,
		}
		if err := json.Unmarshal(resBytes, &response); err != nil {
			return errors.Wrapf(err, "failed decode http result to expected struct, raw http result:%s", string(resBytes))
		}

		return nil
	})
	if err != nil {
		return errors.Wrap(err, "failed to call RPC")
	}

	return nil
}

func (c *RPCClient) doJSON(
	ctx context.Context,
	method string,
	reqBody interface{},
	resDecoder func([]byte) error,
) error {
	const (
		requestTimeout = 5 * time.Second
		doTimeout      = 30 * time.Second
		retryDelay     = 300 * time.Millisecond
	)

	doCtx, doCtxCancel := context.WithTimeout(ctx, doTimeout)
	defer doCtxCancel()
	return retry.Do(doCtx, retryDelay, func() error {
		reqCtx, reqCtxCancel := context.WithTimeout(ctx, requestTimeout)
		defer reqCtxCancel()

		return doJSON(reqCtx, method, c.rpcURL, reqBody, resDecoder)
	})
}

func doJSON(ctx context.Context, method, url string, reqBody interface{}, resDecoder func([]byte) error) error {
	reqBodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return errors.Errorf("failed to marshal request body, err: %v", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(reqBodyBytes))
	if err != nil {
		return errors.Errorf("failed to build the request, err: %v", err)
	}

	// fix for the EOF error
	req.Close = true
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return retry.Retryable(errors.Errorf("failed to perform the request, err: %v", err))
	}

	defer resp.Body.Close()
	bodyData, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.Errorf("failed to read the response body, err: %v", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return retry.Retryable(errors.Errorf("failed to perform request, code: %d, body: %s", resp.StatusCode,
			string(bodyData)))
	}

	return resDecoder(bodyData)
}
