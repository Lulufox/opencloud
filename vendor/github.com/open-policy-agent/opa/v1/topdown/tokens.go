// Copyright 2017 The OPA Authors.  All rights reserved.
// Use of this source code is governed by an Apache2
// license that can be found in the LICENSE file.

package topdown

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"math/big"
	"strings"

	"github.com/open-policy-agent/opa/internal/jwx/jwa"
	"github.com/open-policy-agent/opa/internal/jwx/jwk"
	"github.com/open-policy-agent/opa/internal/jwx/jws"
	"github.com/open-policy-agent/opa/v1/ast"
	"github.com/open-policy-agent/opa/v1/topdown/builtins"
	"github.com/open-policy-agent/opa/v1/topdown/cache"
)

const headerJwt = "JWT"

// JSONWebToken represent the 3 parts (header, payload & signature) of
//
//	a JWT in Base64.
type JSONWebToken struct {
	header        string
	payload       string
	signature     string
	decodedHeader ast.Object
}

// decodeHeader populates the decodedHeader field.
func (token *JSONWebToken) decodeHeader() error {
	result, err := getResult(builtinBase64UrlDecode, ast.StringTerm(token.header))
	if err != nil {
		return fmt.Errorf("JWT header had invalid encoding: %w", err)
	}
	decodedHeader, err := validateJWTHeader(string(result.Value.(ast.String)))
	if err != nil {
		return err
	}
	token.decodedHeader = decodedHeader
	return nil
}

// Implements JWT decoding/validation based on RFC 7519 Section 7.2:
// https://tools.ietf.org/html/rfc7519#section-7.2
// It does no data validation, it merely checks that the given string
// represents a structurally valid JWT. It supports JWTs using JWS compact
// serialization.
func builtinJWTDecode(_ BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	token, err := decodeJWT(operands[0].Value)
	if err != nil {
		return err
	}

	if err = token.decodeHeader(); err != nil {
		return err
	}

	p, err := getResult(builtinBase64UrlDecode, ast.StringTerm(token.payload))
	if err != nil {
		return fmt.Errorf("JWT payload had invalid encoding: %v", err)
	}

	if cty := token.decodedHeader.Get(ast.InternedTerm("cty")); cty != nil {
		ctyVal := string(cty.Value.(ast.String))
		// It is possible for the contents of a token to be another
		// token as a result of nested signing or encryption. To handle
		// the case where we are given a token such as this, we check
		// the content type and recurse on the payload if the content
		// is "JWT".
		// When the payload is itself another encoded JWT, then its
		// contents are quoted (behavior of https://jwt.io/). To fix
		// this, remove leading and trailing quotes.
		if ctyVal == headerJwt {
			p, err = getResult(builtinTrim, p, ast.StringTerm(`"'`))
			if err != nil {
				panic("not reached")
			}
			result, err := getResult(builtinJWTDecode, p)
			if err != nil {
				return err
			}
			return iter(result)
		}
	}

	payload, err := extractJSONObject(string(p.Value.(ast.String)))
	if err != nil {
		return err
	}

	s, err := getResult(builtinBase64UrlDecode, ast.StringTerm(token.signature))
	if err != nil {
		return fmt.Errorf("JWT signature had invalid encoding: %v", err)
	}
	sign := hex.EncodeToString([]byte(s.Value.(ast.String)))

	arr := []*ast.Term{
		ast.NewTerm(token.decodedHeader),
		ast.NewTerm(payload),
		ast.StringTerm(sign),
	}

	return iter(ast.ArrayTerm(arr...))
}

// Implements RS256 JWT signature verification
func builtinJWTVerifyRS256(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := builtinJWTVerifyRSA(bctx, operands[0].Value, operands[1].Value, sha256.New, func(publicKey *rsa.PublicKey, digest []byte, signature []byte) error {
		return rsa.VerifyPKCS1v15(
			publicKey,
			crypto.SHA256,
			digest,
			signature)
	})
	if err == nil {
		return iter(ast.InternedTerm(result))
	}
	return err
}

// Implements RS384 JWT signature verification
func builtinJWTVerifyRS384(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := builtinJWTVerifyRSA(bctx, operands[0].Value, operands[1].Value, sha512.New384, func(publicKey *rsa.PublicKey, digest []byte, signature []byte) error {
		return rsa.VerifyPKCS1v15(
			publicKey,
			crypto.SHA384,
			digest,
			signature)
	})
	if err == nil {
		return iter(ast.InternedTerm(result))
	}
	return err
}

// Implements RS512 JWT signature verification
func builtinJWTVerifyRS512(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := builtinJWTVerifyRSA(bctx, operands[0].Value, operands[1].Value, sha512.New, func(publicKey *rsa.PublicKey, digest []byte, signature []byte) error {
		return rsa.VerifyPKCS1v15(
			publicKey,
			crypto.SHA512,
			digest,
			signature)
	})
	if err == nil {
		return iter(ast.InternedTerm(result))
	}
	return err
}

// Implements PS256 JWT signature verification
func builtinJWTVerifyPS256(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := builtinJWTVerifyRSA(bctx, operands[0].Value, operands[1].Value, sha256.New, func(publicKey *rsa.PublicKey, digest []byte, signature []byte) error {
		return rsa.VerifyPSS(
			publicKey,
			crypto.SHA256,
			digest,
			signature,
			nil)
	})
	if err == nil {
		return iter(ast.InternedTerm(result))
	}
	return err
}

// Implements PS384 JWT signature verification
func builtinJWTVerifyPS384(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := builtinJWTVerifyRSA(bctx, operands[0].Value, operands[1].Value, sha512.New384, func(publicKey *rsa.PublicKey, digest []byte, signature []byte) error {
		return rsa.VerifyPSS(
			publicKey,
			crypto.SHA384,
			digest,
			signature,
			nil)
	})
	if err == nil {
		return iter(ast.InternedTerm(result))
	}
	return err
}

// Implements PS512 JWT signature verification
func builtinJWTVerifyPS512(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := builtinJWTVerifyRSA(bctx, operands[0].Value, operands[1].Value, sha512.New, func(publicKey *rsa.PublicKey, digest []byte, signature []byte) error {
		return rsa.VerifyPSS(
			publicKey,
			crypto.SHA512,
			digest,
			signature,
			nil)
	})
	if err == nil {
		return iter(ast.InternedTerm(result))
	}
	return err
}

// Implements RSA JWT signature verification.
func builtinJWTVerifyRSA(bctx BuiltinContext, jwt ast.Value, keyStr ast.Value, hasher func() hash.Hash, verify func(publicKey *rsa.PublicKey, digest []byte, signature []byte) error) (bool, error) {
	return builtinJWTVerify(bctx, jwt, keyStr, hasher, func(publicKey any, digest []byte, signature []byte) error {
		publicKeyRsa, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			return errors.New("incorrect public key type")
		}
		return verify(publicKeyRsa, digest, signature)
	})
}

// Implements ES256 JWT signature verification.
func builtinJWTVerifyES256(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := builtinJWTVerify(bctx, operands[0].Value, operands[1].Value, sha256.New, verifyES)
	if err == nil {
		return iter(ast.InternedTerm(result))
	}
	return err
}

// Implements ES384 JWT signature verification
func builtinJWTVerifyES384(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := builtinJWTVerify(bctx, operands[0].Value, operands[1].Value, sha512.New384, verifyES)
	if err == nil {
		return iter(ast.InternedTerm(result))
	}
	return err
}

// Implements ES512 JWT signature verification
func builtinJWTVerifyES512(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	result, err := builtinJWTVerify(bctx, operands[0].Value, operands[1].Value, sha512.New, verifyES)
	if err == nil {
		return iter(ast.InternedTerm(result))
	}
	return err
}

func verifyES(publicKey any, digest []byte, signature []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ECDSA signature verification error: %v", r)
		}
	}()
	publicKeyEcdsa, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("incorrect public key type")
	}
	r, s := &big.Int{}, &big.Int{}
	n := len(signature) / 2
	r.SetBytes(signature[:n])
	s.SetBytes(signature[n:])
	if ecdsa.Verify(publicKeyEcdsa, digest, r, s) {
		return nil
	}
	return errors.New("ECDSA signature verification error")
}

type verificationKey struct {
	alg string
	kid string
	key any
}

// getKeysFromCertOrJWK returns the public key found in a X.509 certificate or JWK key(s).
// A valid PEM block is never valid JSON (and vice versa), hence can try parsing both.
// When provided a JWKS, each key additionally likely contains a key ID and the key algorithm.
func getKeysFromCertOrJWK(certificate string) ([]verificationKey, error) {
	if block, rest := pem.Decode([]byte(certificate)); block != nil {
		if len(rest) > 0 {
			return nil, errors.New("extra data after a PEM certificate block")
		}

		if block.Type == blockTypeCertificate {
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse a PEM certificate: %w", err)
			}
			return []verificationKey{{key: cert.PublicKey}}, nil
		}

		if block.Type == "PUBLIC KEY" {
			key, err := x509.ParsePKIXPublicKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("failed to parse a PEM public key: %w", err)
			}

			return []verificationKey{{key: key}}, nil
		}

		return nil, errors.New("failed to extract a Key from the PEM certificate")
	}

	jwks, err := jwk.ParseString(certificate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse a JWK key (set): %w", err)
	}

	keys := make([]verificationKey, 0, len(jwks.Keys))
	for _, k := range jwks.Keys {
		key, err := k.Materialize()
		if err != nil {
			return nil, err
		}
		keys = append(keys, verificationKey{
			alg: k.GetAlgorithm().String(),
			kid: k.GetKeyID(),
			key: key,
		})
	}

	return keys, nil
}

func getKeyByKid(kid string, keys []verificationKey) *verificationKey {
	for _, key := range keys {
		if key.kid == kid {
			return &key
		}
	}
	return nil
}

// Implements JWT signature verification.
func builtinJWTVerify(bctx BuiltinContext, jwt ast.Value, keyStr ast.Value, hasher func() hash.Hash, verify func(publicKey any, digest []byte, signature []byte) error) (bool, error) {
	if found, _, _, valid := getTokenFromCache(bctx, jwt, keyStr); found {
		return valid, nil
	}

	token, err := decodeJWT(jwt)
	if err != nil {
		return false, err
	}

	s, err := builtins.StringOperand(keyStr, 2)
	if err != nil {
		return false, err
	}

	keys, err := getKeysFromCertOrJWK(string(s))
	if err != nil {
		return false, err
	}

	signature, err := token.decodeSignature()
	if err != nil {
		return false, err
	}

	err = token.decodeHeader()
	if err != nil {
		return false, err
	}
	header, err := parseTokenHeader(token)
	if err != nil {
		return false, err
	}

	done := func(valid bool) (bool, error) {
		putTokenInCache(bctx, jwt, keyStr, nil, nil, valid)
		return valid, nil
	}

	// Validate the JWT signature

	// First, check if there's a matching key ID (`kid`) in both token header and key(s).
	// If a match is found, verify using only that key. Only applicable when a JWKS was provided.
	if header.kid != "" {
		if key := getKeyByKid(header.kid, keys); key != nil {
			err = verify(key.key, getInputSHA([]byte(token.header+"."+token.payload), hasher), []byte(signature))

			return done(err == nil)
		}
	}

	// If no key ID matched, try to verify using any key in the set
	// If an alg is present in both the JWT header and the key, skip verification unless they match
	for _, key := range keys {
		if key.alg == "" {
			// No algorithm provided for the key - this is likely a certificate and not a JWKS, so
			// we'll need to verify to find out
			err = verify(key.key, getInputSHA([]byte(token.header+"."+token.payload), hasher), []byte(signature))
			if err == nil {
				return done(true)
			}
		} else {
			if header.alg != key.alg {
				continue
			}
			err = verify(key.key, getInputSHA([]byte(token.header+"."+token.payload), hasher), []byte(signature))
			if err == nil {
				return done(true)
			}
		}
	}

	// None of the keys worked, return false
	return done(false)
}

// Implements HS256 (secret) JWT signature verification
func builtinJWTVerifyHS256(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return builtinJWTVerifyHS(bctx, operands, sha256.New, iter)
}

// Implements HS384 JWT signature verification
func builtinJWTVerifyHS384(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return builtinJWTVerifyHS(bctx, operands, sha512.New384, iter)
}

// Implements HS512 JWT signature verification
func builtinJWTVerifyHS512(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	return builtinJWTVerifyHS(bctx, operands, sha512.New, iter)
}

func builtinJWTVerifyHS(bctx BuiltinContext, operands []*ast.Term, hashF func() hash.Hash, iter func(*ast.Term) error) error {
	jwt, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}

	// Process Secret input
	astSecret, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	if found, _, _, valid := getTokenFromCache(bctx, jwt, astSecret); found {
		return iter(ast.InternedTerm(valid))
	}

	// Decode the JSON Web Token
	token, err := decodeJWT(jwt)
	if err != nil {
		return err
	}

	secret := string(astSecret)

	mac := hmac.New(hashF, []byte(secret))
	_, err = mac.Write([]byte(token.header + "." + token.payload))
	if err != nil {
		return err
	}

	signature, err := token.decodeSignature()
	if err != nil {
		return err
	}

	valid := hmac.Equal([]byte(signature), mac.Sum(nil))

	putTokenInCache(bctx, jwt, astSecret, nil, nil, valid)

	return iter(ast.InternedTerm(valid))
}

// -- Full JWT verification and decoding --

// Verification constraints. See tokens_test.go for unit tests.

// tokenConstraints holds decoded JWT verification constraints.
type tokenConstraints struct {
	// The set of asymmetric keys we can verify with.
	keys []verificationKey

	// The single symmetric key we will verify with.
	secret string

	// The algorithm that must be used to verify.
	// If "", any algorithm is acceptable.
	alg string

	// The required issuer.
	// If "", any issuer is acceptable.
	iss string

	// The required audience.
	// If "", no audience is acceptable.
	aud string

	// The time to validate against, or -1 if no constraint set.
	// (If unset, the current time will be used.)
	time float64
}

// tokenConstraintHandler is the handler type for JWT verification constraints.
type tokenConstraintHandler func(value ast.Value, parameters *tokenConstraints) error

// tokenConstraintTypes maps known JWT verification constraints to handlers.
var tokenConstraintTypes = map[string]tokenConstraintHandler{
	"cert": tokenConstraintCert,
	"secret": func(value ast.Value, constraints *tokenConstraints) error {
		return tokenConstraintString("secret", value, &constraints.secret)
	},
	"alg": func(value ast.Value, constraints *tokenConstraints) error {
		return tokenConstraintString("alg", value, &constraints.alg)
	},
	"iss": func(value ast.Value, constraints *tokenConstraints) error {
		return tokenConstraintString("iss", value, &constraints.iss)
	},
	"aud": func(value ast.Value, constraints *tokenConstraints) error {
		return tokenConstraintString("aud", value, &constraints.aud)
	},
	"time": tokenConstraintTime,
}

// tokenConstraintCert handles the `cert` constraint.
func tokenConstraintCert(value ast.Value, constraints *tokenConstraints) error {
	s, ok := value.(ast.String)
	if !ok {
		return errors.New("cert constraint: must be a string")
	}

	keys, err := getKeysFromCertOrJWK(string(s))
	if err != nil {
		return err
	}

	constraints.keys = keys
	return nil
}

// tokenConstraintTime handles the `time` constraint.
func tokenConstraintTime(value ast.Value, constraints *tokenConstraints) error {
	t, err := timeFromValue(value)
	if err != nil {
		return err
	}
	constraints.time = t
	return nil
}

func timeFromValue(value ast.Value) (float64, error) {
	time, ok := value.(ast.Number)
	if !ok {
		return 0, errors.New("token time constraint: must be a number")
	}
	timeFloat, ok := time.Float64()
	if !ok {
		return 0, errors.New("token time constraint: unvalid float64")
	}
	if timeFloat < 0 {
		return 0, errors.New("token time constraint: must not be negative")
	}
	return timeFloat, nil
}

// tokenConstraintString handles string constraints.
func tokenConstraintString(name string, value ast.Value, where *string) error {
	av, ok := value.(ast.String)
	if !ok {
		return fmt.Errorf("%s constraint: must be a string", name)
	}
	*where = string(av)
	return nil
}

// parseTokenConstraints parses the constraints argument.
func parseTokenConstraints(o ast.Object, wallclock *ast.Term) (*tokenConstraints, error) {
	constraints := tokenConstraints{
		time: -1,
	}
	if err := o.Iter(func(k *ast.Term, v *ast.Term) error {
		name := string(k.Value.(ast.String))
		handler, ok := tokenConstraintTypes[name]
		if ok {
			return handler(v.Value, &constraints)
		}
		// Anything unknown is rejected.
		return fmt.Errorf("unknown token validation constraint: %s", name)
	}); err != nil {
		return nil, err
	}
	if constraints.time == -1 { // no time provided in constraint object
		t, err := timeFromValue(wallclock.Value)
		if err != nil {
			return nil, err
		}
		constraints.time = t
	}
	return &constraints, nil
}

// validate validates the constraints argument.
func (constraints *tokenConstraints) validate() error {
	keys := 0
	if constraints.keys != nil {
		keys++
	}
	if constraints.secret != "" {
		keys++
	}
	if keys > 1 {
		return errors.New("duplicate key constraints")
	}
	if keys < 1 {
		return errors.New("no key constraint")
	}
	return nil
}

// verify verifies a JWT using the constraints and the algorithm from the header
func (constraints *tokenConstraints) verify(kid, alg, header, payload, signature string) error {
	// Construct the payload
	plaintext := []byte(header)
	plaintext = append(plaintext, []byte(".")...)
	plaintext = append(plaintext, payload...)
	// Look up the algorithm
	a, ok := tokenAlgorithms[alg]
	if !ok {
		return fmt.Errorf("unknown JWS algorithm: %s", alg)
	}
	// If we're configured with asymmetric key(s) then only trust that
	if constraints.keys != nil {
		if kid != "" {
			if key := getKeyByKid(kid, constraints.keys); key != nil {
				err := a.verify(key.key, a.hash, plaintext, []byte(signature))
				if err != nil {
					return errSignatureNotVerified
				}
				return nil
			}
		}

		verified := false
		for _, key := range constraints.keys {
			if key.alg == "" {
				err := a.verify(key.key, a.hash, plaintext, []byte(signature))
				if err == nil {
					verified = true
					break
				}
			} else {
				if alg != key.alg {
					continue
				}
				err := a.verify(key.key, a.hash, plaintext, []byte(signature))
				if err == nil {
					verified = true
					break
				}
			}
		}

		if !verified {
			return errSignatureNotVerified
		}
		return nil
	}
	if constraints.secret != "" {
		return a.verify([]byte(constraints.secret), a.hash, plaintext, []byte(signature))
	}
	// (*tokenConstraints)validate() should prevent this happening
	return errors.New("unexpectedly found no keys to trust")
}

// validAudience checks the audience of the JWT.
// It returns true if it meets the constraints and false otherwise.
func (constraints *tokenConstraints) validAudience(aud ast.Value) bool {
	s, ok := aud.(ast.String)
	if ok {
		return string(s) == constraints.aud
	}
	a, ok := aud.(*ast.Array)
	if !ok {
		return false
	}
	return a.Until(func(elem *ast.Term) bool {
		if s, ok := elem.Value.(ast.String); ok {
			return string(s) == constraints.aud
		}
		return false
	})
}

// JWT algorithms

type (
	tokenVerifyFunction           func(key any, hash crypto.Hash, payload []byte, signature []byte) error
	tokenVerifyAsymmetricFunction func(key any, hash crypto.Hash, digest []byte, signature []byte) error
)

// jwtAlgorithm describes a JWS 'alg' value
type tokenAlgorithm struct {
	hash   crypto.Hash
	verify tokenVerifyFunction
}

// tokenAlgorithms is the known JWT algorithms
var tokenAlgorithms = map[string]tokenAlgorithm{
	"RS256": {crypto.SHA256, verifyAsymmetric(verifyRSAPKCS)},
	"RS384": {crypto.SHA384, verifyAsymmetric(verifyRSAPKCS)},
	"RS512": {crypto.SHA512, verifyAsymmetric(verifyRSAPKCS)},
	"PS256": {crypto.SHA256, verifyAsymmetric(verifyRSAPSS)},
	"PS384": {crypto.SHA384, verifyAsymmetric(verifyRSAPSS)},
	"PS512": {crypto.SHA512, verifyAsymmetric(verifyRSAPSS)},
	"ES256": {crypto.SHA256, verifyAsymmetric(verifyECDSA)},
	"ES384": {crypto.SHA384, verifyAsymmetric(verifyECDSA)},
	"ES512": {crypto.SHA512, verifyAsymmetric(verifyECDSA)},
	"HS256": {crypto.SHA256, verifyHMAC},
	"HS384": {crypto.SHA384, verifyHMAC},
	"HS512": {crypto.SHA512, verifyHMAC},
}

// errSignatureNotVerified is returned when a signature cannot be verified.
var errSignatureNotVerified = errors.New("signature not verified")

func verifyHMAC(key any, hash crypto.Hash, payload []byte, signature []byte) error {
	macKey, ok := key.([]byte)
	if !ok {
		return errors.New("incorrect symmetric key type")
	}
	mac := hmac.New(hash.New, macKey)
	if _, err := mac.Write(payload); err != nil {
		return err
	}
	if !hmac.Equal(signature, mac.Sum([]byte{})) {
		return errSignatureNotVerified
	}
	return nil
}

func verifyAsymmetric(verify tokenVerifyAsymmetricFunction) tokenVerifyFunction {
	return func(key any, hash crypto.Hash, payload []byte, signature []byte) error {
		h := hash.New()
		h.Write(payload)
		return verify(key, hash, h.Sum([]byte{}), signature)
	}
}

func verifyRSAPKCS(key any, hash crypto.Hash, digest []byte, signature []byte) error {
	publicKeyRsa, ok := key.(*rsa.PublicKey)
	if !ok {
		return errors.New("incorrect public key type")
	}
	if err := rsa.VerifyPKCS1v15(publicKeyRsa, hash, digest, signature); err != nil {
		return errSignatureNotVerified
	}
	return nil
}

func verifyRSAPSS(key any, hash crypto.Hash, digest []byte, signature []byte) error {
	publicKeyRsa, ok := key.(*rsa.PublicKey)
	if !ok {
		return errors.New("incorrect public key type")
	}
	if err := rsa.VerifyPSS(publicKeyRsa, hash, digest, signature, nil); err != nil {
		return errSignatureNotVerified
	}
	return nil
}

func verifyECDSA(key any, _ crypto.Hash, digest []byte, signature []byte) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("ECDSA signature verification error: %v", r)
		}
	}()
	publicKeyEcdsa, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("incorrect public key type")
	}
	r, s := &big.Int{}, &big.Int{}
	n := len(signature) / 2
	r.SetBytes(signature[:n])
	s.SetBytes(signature[n:])
	if ecdsa.Verify(publicKeyEcdsa, digest, r, s) {
		return nil
	}
	return errSignatureNotVerified
}

// JWT header parsing and parameters. See tokens_test.go for unit tests.

// tokenHeaderType represents a recognized JWT header field
// tokenHeader is a parsed JWT header
type tokenHeader struct {
	alg     string
	kid     string
	typ     string
	cty     string
	crit    map[string]bool
	unknown []string
}

// tokenHeaderHandler handles a JWT header parameters
type tokenHeaderHandler func(header *tokenHeader, value ast.Value) error

// tokenHeaderTypes maps known JWT header parameters to handlers
var tokenHeaderTypes = map[string]tokenHeaderHandler{
	"alg": func(header *tokenHeader, value ast.Value) error {
		return tokenHeaderString("alg", &header.alg, value)
	},
	"kid": func(header *tokenHeader, value ast.Value) error {
		return tokenHeaderString("kid", &header.kid, value)
	},
	"typ": func(header *tokenHeader, value ast.Value) error {
		return tokenHeaderString("typ", &header.typ, value)
	},
	"cty": func(header *tokenHeader, value ast.Value) error {
		return tokenHeaderString("cty", &header.cty, value)
	},
	"crit": tokenHeaderCrit,
}

// tokenHeaderCrit handles the 'crit' header parameter
func tokenHeaderCrit(header *tokenHeader, value ast.Value) error {
	v, ok := value.(*ast.Array)
	if !ok {
		return errors.New("crit: must be a list")
	}
	header.crit = map[string]bool{}
	_ = v.Iter(func(elem *ast.Term) error {
		tv, ok := elem.Value.(ast.String)
		if !ok {
			return errors.New("crit: must be a list of strings")
		}
		header.crit[string(tv)] = true
		return nil
	})
	if len(header.crit) == 0 {
		return errors.New("crit: must be a nonempty list") // 'MUST NOT' use the empty list
	}
	return nil
}

// tokenHeaderString handles string-format JWT header parameters
func tokenHeaderString(name string, where *string, value ast.Value) error {
	v, ok := value.(ast.String)
	if !ok {
		return fmt.Errorf("%s: must be a string", name)
	}
	*where = string(v)
	return nil
}

// parseTokenHeader parses the JWT header.
func parseTokenHeader(token *JSONWebToken) (*tokenHeader, error) {
	header := tokenHeader{
		unknown: []string{},
	}
	if err := token.decodedHeader.Iter(func(k *ast.Term, v *ast.Term) error {
		ks := string(k.Value.(ast.String))
		handler, ok := tokenHeaderTypes[ks]
		if !ok {
			header.unknown = append(header.unknown, ks)
			return nil
		}
		return handler(&header, v.Value)
	}); err != nil {
		return nil, err
	}
	return &header, nil
}

// validTokenHeader returns true if the JOSE header is valid, otherwise false.
func (header *tokenHeader) valid() bool {
	// RFC7515 s4.1.1 alg MUST be present
	if header.alg == "" {
		return false
	}
	// RFC7515 4.1.11 JWS is invalid if there is a critical parameter that we did not recognize
	for _, u := range header.unknown {
		if header.crit[u] {
			return false
		}
	}
	return true
}

func commonBuiltinJWTEncodeSign(bctx BuiltinContext, inputHeaders, jwsPayload, jwkSrc string, iter func(*ast.Term) error) error {
	keys, err := jwk.ParseString(jwkSrc)
	if err != nil {
		return err
	}
	key, err := keys.Keys[0].Materialize()
	if err != nil {
		return err
	}
	if jwk.GetKeyTypeFromKey(key) != keys.Keys[0].GetKeyType() {
		return errors.New("JWK derived key type and keyType parameter do not match")
	}

	standardHeaders := &jws.StandardHeaders{}
	jwsHeaders := []byte(inputHeaders)
	err = json.Unmarshal(jwsHeaders, standardHeaders)
	if err != nil {
		return err
	}
	alg := standardHeaders.GetAlgorithm()
	if alg == jwa.Unsupported {
		return errors.New("unknown signature algorithm")
	}

	if (standardHeaders.Type == "" || standardHeaders.Type == headerJwt) && !json.Valid([]byte(jwsPayload)) {
		return errors.New("type is JWT but payload is not JSON")
	}

	// process payload and sign
	var jwsCompact []byte
	jwsCompact, err = jws.SignLiteral([]byte(jwsPayload), alg, key, jwsHeaders, bctx.Seed)
	if err != nil {
		return err
	}

	return iter(ast.StringTerm(string(jwsCompact)))
}

func builtinJWTEncodeSign(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	inputHeadersAsJSON, err := ast.JSON(operands[0].Value)
	if err != nil {
		return fmt.Errorf("failed to prepare JWT headers for marshalling: %v", err)
	}

	inputHeadersBs, err := json.Marshal(inputHeadersAsJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal JWT headers: %v", err)
	}

	payloadAsJSON, err := ast.JSON(operands[1].Value)
	if err != nil {
		return fmt.Errorf("failed to prepare JWT payload for marshalling: %v", err)
	}

	payloadBs, err := json.Marshal(payloadAsJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal JWT payload: %v", err)
	}

	signatureAsJSON, err := ast.JSON(operands[2].Value)
	if err != nil {
		return fmt.Errorf("failed to prepare JWT signature for marshalling: %v", err)
	}

	signatureBs, err := json.Marshal(signatureAsJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal JWT signature: %v", err)
	}

	return commonBuiltinJWTEncodeSign(
		bctx,
		string(inputHeadersBs),
		string(payloadBs),
		string(signatureBs),
		iter,
	)
}

func builtinJWTEncodeSignRaw(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	jwkSrc, err := builtins.StringOperand(operands[2].Value, 3)
	if err != nil {
		return err
	}
	inputHeaders, err := builtins.StringOperand(operands[0].Value, 1)
	if err != nil {
		return err
	}
	jwsPayload, err := builtins.StringOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}
	return commonBuiltinJWTEncodeSign(bctx, string(inputHeaders), string(jwsPayload), string(jwkSrc), iter)
}

// Implements full JWT decoding, validation and verification.
func builtinJWTDecodeVerify(bctx BuiltinContext, operands []*ast.Term, iter func(*ast.Term) error) error {
	// io.jwt.decode_verify(string, constraints, [valid, header, payload])
	//
	// If valid is true then the signature verifies and all constraints are met.
	// If valid is false then either the signature did not verify or some constrain
	// was not met.
	//
	// Decoding errors etc are returned as errors.
	a := operands[0].Value

	b, err := builtins.ObjectOperand(operands[1].Value, 2)
	if err != nil {
		return err
	}

	unverified := ast.ArrayTerm(
		ast.InternedTerm(false),
		ast.InternedEmptyObject,
		ast.InternedEmptyObject,
	)
	constraints, err := parseTokenConstraints(b, bctx.Time)
	if err != nil {
		return err
	}
	if err := constraints.validate(); err != nil {
		return err
	}
	var token *JSONWebToken
	var payload ast.Object
	var header ast.Object

	// FIXME: optimize
	k, _ := b.Filter(ast.NewObject(
		ast.Item(ast.InternedTerm("secret"), ast.InternedEmptyObject),
		ast.Item(ast.InternedTerm("cert"), ast.InternedEmptyObject),
	))

	if found, th, tp, validSignature := getTokenFromCache(bctx, a, k); found {
		if !validSignature {
			// For the given token and key(s), the signature is invalid
			return iter(unverified)
		}

		if th != nil && tp != nil {
			header = th
			payload = tp
		} else {
			// Cache entry was created by one of the other built-ins that doesn't decode header/payload

			if token, err = decodeJWT(a); err != nil {
				return err
			}

			header = token.decodedHeader

			p, err := getResult(builtinBase64UrlDecode, ast.StringTerm(token.payload))
			if err != nil {
				return fmt.Errorf("JWT payload had invalid encoding: %v", err)
			}

			payload, err = extractJSONObject(string(p.Value.(ast.String)))
			if err != nil {
				return err
			}

			putTokenInCache(bctx, a, k, header, payload, true)
		}
	} else {
		var p *ast.Term

		for {
			// RFC7519 7.2 #1-2 split into parts
			if token, err = decodeJWT(a); err != nil {
				return err
			}

			// RFC7519 7.2 #3, #4, #6
			if err := token.decodeHeader(); err != nil {
				return err
			}

			// RFC7159 7.2 #5 (and RFC7159 5.2 #5) validate header fields
			header, err := parseTokenHeader(token)
			if err != nil {
				return err
			}

			if !header.valid() {
				return iter(unverified)
			}

			// Check constraints that impact signature verification.
			if constraints.alg != "" && constraints.alg != header.alg {
				return iter(unverified)
			}

			// RFC7159 7.2 #7 verify the signature
			signature, err := token.decodeSignature()
			if err != nil {
				return err
			}

			if err := constraints.verify(header.kid, header.alg, token.header, token.payload, signature); err != nil {
				if err == errSignatureNotVerified {
					putTokenInCache(bctx, a, k, nil, nil, false)
					return iter(unverified)
				}
				return err
			}

			// RFC7159 7.2 #9-10 decode the payload
			p, err = getResult(builtinBase64UrlDecode, ast.StringTerm(token.payload))
			if err != nil {
				return fmt.Errorf("JWT payload had invalid encoding: %v", err)
			}

			// RFC7159 7.2 #8 and 5.2 cty
			if strings.EqualFold(header.cty, headerJwt) {
				// Nested JWT, go round again with payload as first argument
				a = p.Value
				continue
			}

			// Non-nested JWT (or we've reached the bottom of the nesting).
			break
		}

		payload, err = extractJSONObject(string(p.Value.(ast.String)))
		if err != nil {
			return err
		}

		header = token.decodedHeader

		putTokenInCache(bctx, a, k, header, payload, true)
	}

	// Check registered claim names against constraints or environment
	// RFC7159 4.1.1 iss
	if constraints.iss != "" {
		if iss := payload.Get(ast.InternedTerm("iss")); iss != nil {
			issVal := string(iss.Value.(ast.String))
			if constraints.iss != issVal {
				return iter(unverified)
			}
		} else {
			return iter(unverified)
		}
	}
	// RFC7159 4.1.3 aud
	if aud := payload.Get(ast.InternedTerm("aud")); aud != nil {
		if !constraints.validAudience(aud.Value) {
			return iter(unverified)
		}
	} else {
		if constraints.aud != "" {
			return iter(unverified)
		}
	}
	// RFC7159 4.1.4 exp
	if exp := payload.Get(ast.InternedTerm("exp")); exp != nil {
		switch v := exp.Value.(type) {
		case ast.Number:
			// constraints.time is in nanoseconds but exp Value is in seconds
			compareTime := ast.FloatNumberTerm(constraints.time / 1000000000)
			if ast.Compare(compareTime, v) != -1 {
				return iter(unverified)
			}
		default:
			return errors.New("exp value must be a number")
		}
	}
	// RFC7159 4.1.5 nbf
	if nbf := payload.Get(ast.InternedTerm("nbf")); nbf != nil {
		switch v := nbf.Value.(type) {
		case ast.Number:
			// constraints.time is in nanoseconds but nbf Value is in seconds
			compareTime := ast.FloatNumberTerm(constraints.time / 1000000000)
			if ast.Compare(compareTime, v) == -1 {
				return iter(unverified)
			}
		default:
			return errors.New("nbf value must be a number")
		}
	}

	verified := ast.ArrayTerm(
		ast.InternedTerm(true),
		ast.NewTerm(header),
		ast.NewTerm(payload),
	)
	return iter(verified)
}

// -- Utilities --

func decodeJWT(a ast.Value) (*JSONWebToken, error) {
	// Parse the JSON Web Token
	astEncode, err := builtins.StringOperand(a, 1)
	if err != nil {
		return nil, err
	}

	encoding := string(astEncode)
	if !strings.Contains(encoding, ".") {
		return nil, errors.New("encoded JWT had no period separators")
	}

	parts := strings.Split(encoding, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("encoded JWT must have 3 sections, found %d", len(parts))
	}

	return &JSONWebToken{header: parts[0], payload: parts[1], signature: parts[2]}, nil
}

func (token *JSONWebToken) decodeSignature() (string, error) {
	decodedSignature, err := getResult(builtinBase64UrlDecode, ast.StringTerm(token.signature))
	if err != nil {
		return "", err
	}

	signatureAst, err := builtins.StringOperand(decodedSignature.Value, 1)
	if err != nil {
		return "", err
	}
	return string(signatureAst), err
}

// Extract, validate and return the JWT header as an ast.Object.
func validateJWTHeader(h string) (ast.Object, error) {
	header, err := extractJSONObject(h)
	if err != nil {
		return nil, fmt.Errorf("bad JWT header: %v", err)
	}

	// There are two kinds of JWT tokens, a JSON Web Signature (JWS) and
	// a JSON Web Encryption (JWE). The latter is very involved, and we
	// won't support it for now.
	// This code checks which kind of JWT we are dealing with according to
	// RFC 7516 Section 9: https://tools.ietf.org/html/rfc7516#section-9
	if header.Get(ast.InternedTerm("enc")) != nil {
		return nil, errors.New("JWT is a JWE object, which is not supported")
	}

	return header, nil
}

func extractJSONObject(s string) (ast.Object, error) {
	// XXX: This code relies on undocumented behavior of Go's
	// json.Unmarshal using the last occurrence of duplicate keys in a JSON
	// Object. If duplicate keys are present in a JWT, the last must be
	// used or the token rejected. Since detecting duplicates is tantamount
	// to parsing it ourselves, we're relying on the Go implementation
	// using the last occurring instance of the key, which is the behavior
	// as of Go 1.8.1.
	v, err := getResult(builtinJSONUnmarshal, ast.StringTerm(s))
	if err != nil {
		return nil, fmt.Errorf("invalid JSON: %v", err)
	}

	o, ok := v.Value.(ast.Object)
	if !ok {
		return nil, errors.New("decoded JSON type was not an Object")
	}

	return o, nil
}

// getInputSha returns the SHA checksum of the input
func getInputSHA(input []byte, h func() hash.Hash) []byte {
	hasher := h()
	hasher.Write(input)
	return hasher.Sum(nil)
}

type jwtCacheEntry struct {
	payload        ast.Object
	header         ast.Object
	validSignature bool
}

const tokenCacheName = "io_jwt"

func getTokenFromCache(bctx BuiltinContext, serializedJwt ast.Value, publicKey ast.Value) (bool, ast.Object, ast.Object, bool) {
	if bctx.InterQueryBuiltinValueCache == nil {
		return false, nil, nil, false
	}

	c := bctx.InterQueryBuiltinValueCache.GetCache(tokenCacheName)
	if c == nil {
		return false, nil, nil, false
	}

	key := createTokenCacheKey(serializedJwt, publicKey)

	entry, ok := c.Get(key)
	if !ok {
		return false, nil, nil, false
	}

	if jwtEntry, ok := entry.(jwtCacheEntry); ok {
		return true, jwtEntry.header, jwtEntry.payload, jwtEntry.validSignature
	}

	return false, nil, nil, false
}

func putTokenInCache(bctx BuiltinContext, serializedJwt ast.Value, publicKey ast.Value, header ast.Object, payload ast.Object, validSignature bool) {
	if bctx.InterQueryBuiltinValueCache == nil {
		return
	}

	c := bctx.InterQueryBuiltinValueCache.GetCache(tokenCacheName)
	if c == nil {
		return
	}

	key := createTokenCacheKey(serializedJwt, publicKey)

	c.Insert(key, jwtCacheEntry{header: header, payload: payload, validSignature: validSignature})
}

func createTokenCacheKey(serializedJwt ast.Value, publicKey ast.Value) ast.Value {
	// We need to create a key that is unique to the serialized JWT (for lookup) and the public key used to verify it,
	// so that we don't get a misleading cached validation result for a different, invalid key.
	return ast.NewArray(ast.NewTerm(serializedJwt), ast.NewTerm(publicKey))
}

func init() {
	// By default, the JWT cache is disabled.
	cache.RegisterDefaultInterQueryBuiltinValueCacheConfig(tokenCacheName, nil)

	RegisterBuiltinFunc(ast.JWTDecode.Name, builtinJWTDecode)
	RegisterBuiltinFunc(ast.JWTVerifyRS256.Name, builtinJWTVerifyRS256)
	RegisterBuiltinFunc(ast.JWTVerifyRS384.Name, builtinJWTVerifyRS384)
	RegisterBuiltinFunc(ast.JWTVerifyRS512.Name, builtinJWTVerifyRS512)
	RegisterBuiltinFunc(ast.JWTVerifyPS256.Name, builtinJWTVerifyPS256)
	RegisterBuiltinFunc(ast.JWTVerifyPS384.Name, builtinJWTVerifyPS384)
	RegisterBuiltinFunc(ast.JWTVerifyPS512.Name, builtinJWTVerifyPS512)
	RegisterBuiltinFunc(ast.JWTVerifyES256.Name, builtinJWTVerifyES256)
	RegisterBuiltinFunc(ast.JWTVerifyES384.Name, builtinJWTVerifyES384)
	RegisterBuiltinFunc(ast.JWTVerifyES512.Name, builtinJWTVerifyES512)
	RegisterBuiltinFunc(ast.JWTVerifyHS256.Name, builtinJWTVerifyHS256)
	RegisterBuiltinFunc(ast.JWTVerifyHS384.Name, builtinJWTVerifyHS384)
	RegisterBuiltinFunc(ast.JWTVerifyHS512.Name, builtinJWTVerifyHS512)
	RegisterBuiltinFunc(ast.JWTDecodeVerify.Name, builtinJWTDecodeVerify)
	RegisterBuiltinFunc(ast.JWTEncodeSignRaw.Name, builtinJWTEncodeSignRaw)
	RegisterBuiltinFunc(ast.JWTEncodeSign.Name, builtinJWTEncodeSign)
}
