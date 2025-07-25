/*
Libre Graph API

Libre Graph is a free API for cloud collaboration inspired by the MS Graph API.

API version: v1.0.8
*/

// Code generated by OpenAPI Generator (https://openapi-generator.tech); DO NOT EDIT.

package libregraph

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
)


// MeDrivesApiService MeDrivesApi service
type MeDrivesApiService service

type ApiListMyDrivesRequest struct {
	ctx context.Context
	ApiService *MeDrivesApiService
	orderby *string
	filter *string
}

// The $orderby system query option allows clients to request resources in either ascending order using asc or descending order using desc.
func (r ApiListMyDrivesRequest) Orderby(orderby string) ApiListMyDrivesRequest {
	r.orderby = &orderby
	return r
}

// Filter items by property values
func (r ApiListMyDrivesRequest) Filter(filter string) ApiListMyDrivesRequest {
	r.filter = &filter
	return r
}

func (r ApiListMyDrivesRequest) Execute() (*CollectionOfDrives, *http.Response, error) {
	return r.ApiService.ListMyDrivesExecute(r)
}

/*
ListMyDrives Get all drives where the current user is a regular member of

 @param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 @return ApiListMyDrivesRequest
*/
func (a *MeDrivesApiService) ListMyDrives(ctx context.Context) ApiListMyDrivesRequest {
	return ApiListMyDrivesRequest{
		ApiService: a,
		ctx: ctx,
	}
}

// Execute executes the request
//  @return CollectionOfDrives
func (a *MeDrivesApiService) ListMyDrivesExecute(r ApiListMyDrivesRequest) (*CollectionOfDrives, *http.Response, error) {
	var (
		localVarHTTPMethod   = http.MethodGet
		localVarPostBody     interface{}
		formFiles            []formFile
		localVarReturnValue  *CollectionOfDrives
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "MeDrivesApiService.ListMyDrives")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1.0/me/drives"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}

	if r.orderby != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "$orderby", r.orderby, "form", "")
	}
	if r.filter != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "$filter", r.filter, "form", "")
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
			var v OdataError
			err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
			if err != nil {
				newErr.error = err.Error()
				return localVarReturnValue, localVarHTTPResponse, newErr
			}
					newErr.error = formatErrorMessage(localVarHTTPResponse.Status, &v)
					newErr.model = v
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}

type ApiListMyDrivesBetaRequest struct {
	ctx context.Context
	ApiService *MeDrivesApiService
	orderby *string
	filter *string
	expand *string
	select_ *[]string
}

// The $orderby system query option allows clients to request resources in either ascending order using asc or descending order using desc.
func (r ApiListMyDrivesBetaRequest) Orderby(orderby string) ApiListMyDrivesBetaRequest {
	r.orderby = &orderby
	return r
}

// Filter items by property values
func (r ApiListMyDrivesBetaRequest) Filter(filter string) ApiListMyDrivesBetaRequest {
	r.filter = &filter
	return r
}

// Expand related entities
func (r ApiListMyDrivesBetaRequest) Expand(expand string) ApiListMyDrivesBetaRequest {
	r.expand = &expand
	return r
}

// Select properties to be returned. By default all properties are returned.
func (r ApiListMyDrivesBetaRequest) Select_(select_ []string) ApiListMyDrivesBetaRequest {
	r.select_ = &select_
	return r
}

func (r ApiListMyDrivesBetaRequest) Execute() (*CollectionOfDrives, *http.Response, error) {
	return r.ApiService.ListMyDrivesBetaExecute(r)
}

/*
ListMyDrivesBeta Alias for '/v1.0/drives', the difference is that grantedtoV2 is used and roles contain unified roles instead of cs3 roles

 @param ctx context.Context - for authentication, logging, cancellation, deadlines, tracing, etc. Passed from http.Request or context.Background().
 @return ApiListMyDrivesBetaRequest
*/
func (a *MeDrivesApiService) ListMyDrivesBeta(ctx context.Context) ApiListMyDrivesBetaRequest {
	return ApiListMyDrivesBetaRequest{
		ApiService: a,
		ctx: ctx,
	}
}

// Execute executes the request
//  @return CollectionOfDrives
func (a *MeDrivesApiService) ListMyDrivesBetaExecute(r ApiListMyDrivesBetaRequest) (*CollectionOfDrives, *http.Response, error) {
	var (
		localVarHTTPMethod   = http.MethodGet
		localVarPostBody     interface{}
		formFiles            []formFile
		localVarReturnValue  *CollectionOfDrives
	)

	localBasePath, err := a.client.cfg.ServerURLWithContext(r.ctx, "MeDrivesApiService.ListMyDrivesBeta")
	if err != nil {
		return localVarReturnValue, nil, &GenericOpenAPIError{error: err.Error()}
	}

	localVarPath := localBasePath + "/v1beta1/me/drives"

	localVarHeaderParams := make(map[string]string)
	localVarQueryParams := url.Values{}
	localVarFormParams := url.Values{}

	if r.orderby != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "$orderby", r.orderby, "form", "")
	}
	if r.filter != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "$filter", r.filter, "form", "")
	}
	if r.expand != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "$expand", r.expand, "form", "")
	}
	if r.select_ != nil {
		parameterAddToHeaderOrQuery(localVarQueryParams, "$select", r.select_, "form", "csv")
	}
	// to determine the Content-Type header
	localVarHTTPContentTypes := []string{}

	// set Content-Type header
	localVarHTTPContentType := selectHeaderContentType(localVarHTTPContentTypes)
	if localVarHTTPContentType != "" {
		localVarHeaderParams["Content-Type"] = localVarHTTPContentType
	}

	// to determine the Accept header
	localVarHTTPHeaderAccepts := []string{"application/json"}

	// set Accept header
	localVarHTTPHeaderAccept := selectHeaderAccept(localVarHTTPHeaderAccepts)
	if localVarHTTPHeaderAccept != "" {
		localVarHeaderParams["Accept"] = localVarHTTPHeaderAccept
	}
	req, err := a.client.prepareRequest(r.ctx, localVarPath, localVarHTTPMethod, localVarPostBody, localVarHeaderParams, localVarQueryParams, localVarFormParams, formFiles)
	if err != nil {
		return localVarReturnValue, nil, err
	}

	localVarHTTPResponse, err := a.client.callAPI(req)
	if err != nil || localVarHTTPResponse == nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	localVarBody, err := io.ReadAll(localVarHTTPResponse.Body)
	localVarHTTPResponse.Body.Close()
	localVarHTTPResponse.Body = io.NopCloser(bytes.NewBuffer(localVarBody))
	if err != nil {
		return localVarReturnValue, localVarHTTPResponse, err
	}

	if localVarHTTPResponse.StatusCode >= 300 {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: localVarHTTPResponse.Status,
		}
			var v OdataError
			err = a.client.decode(&v, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
			if err != nil {
				newErr.error = err.Error()
				return localVarReturnValue, localVarHTTPResponse, newErr
			}
					newErr.error = formatErrorMessage(localVarHTTPResponse.Status, &v)
					newErr.model = v
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	err = a.client.decode(&localVarReturnValue, localVarBody, localVarHTTPResponse.Header.Get("Content-Type"))
	if err != nil {
		newErr := &GenericOpenAPIError{
			body:  localVarBody,
			error: err.Error(),
		}
		return localVarReturnValue, localVarHTTPResponse, newErr
	}

	return localVarReturnValue, localVarHTTPResponse, nil
}
