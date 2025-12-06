package websub

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySignature_SHA1(t *testing.T) {
	payload := []byte("test payload")
	secret := "test-secret"

	// Generate a valid SHA1 signature
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)
	signature := "sha1=" + hex.EncodeToString(expectedMAC)

	// Verify the signature
	if !VerifySignature(payload, signature, secret) {
		t.Error("Expected SHA1 signature to be valid")
	}
}

func TestVerifySignature_SHA256(t *testing.T) {
	payload := []byte("test payload")
	secret := "test-secret"

	// Generate a valid SHA256 signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)
	signature := "sha256=" + hex.EncodeToString(expectedMAC)

	// Verify the signature
	if !VerifySignature(payload, signature, secret) {
		t.Error("Expected SHA256 signature to be valid")
	}
}

func TestVerifySignature_InvalidSignature(t *testing.T) {
	payload := []byte("test payload")
	secret := "test-secret"
	invalidSignature := "sha1=invalid"

	if VerifySignature(payload, invalidSignature, secret) {
		t.Error("Expected invalid signature to fail verification")
	}
}

func TestVerifySignature_EmptySignature(t *testing.T) {
	payload := []byte("test payload")
	secret := "test-secret"

	if VerifySignature(payload, "", secret) {
		t.Error("Expected empty signature to fail verification")
	}
}

func TestVerifySignature_EmptySecret(t *testing.T) {
	payload := []byte("test payload")
	signature := "sha1=abc123"

	if VerifySignature(payload, signature, "") {
		t.Error("Expected empty secret to fail verification")
	}
}

func TestVerifySignature_RealWorldExample(t *testing.T) {
	// This is based on actual YouTube WebSub notification data from the logs
	// The payload is the XML body that YouTube sends
	payload := []byte(`<?xml version='1.0' encoding='UTF-8'?>
<feed xmlns:yt="http://www.youtube.com/xml/schemas/2015" xmlns="http://www.w3.org/2005/Atom"><link rel="hub" href="https://pubsubhubbub.appspot.com"/><link rel="self" href="https://www.youtube.com/xml/feeds/videos.xml?channel_id=UCkCCgmOBqkezhKIXJHq89zw"/><title>YouTube video feed</title><updated>2025-12-05T23:03:30.889357433+00:00</updated><entry>
  <id>yt:video:v3JMDEjXtLE</id>
  <yt:videoId>v3JMDEjXtLE</yt:videoId>
  <yt:channelId>UCkCCgmOBqkezhKIXJHq89zw</yt:channelId>
  <title>SaturdACE Super Powers!</title>
  <link rel="alternate" href="https://www.youtube.com/watch?v=v3JMDEjXtLE"/>
  <author>
   <name>ACE Console Repairs 🇦🇺</name>
   <uri>https://www.youtube.com/channel/UCkCCgmOBqkezhKIXJHq89zw</uri>
  </author>
  <published>2025-12-05T23:03:27+00:00</published>
  <updated>2025-12-05T23:03:30.889357433+00:00</updated>
 </entry></feed>
`)

	// Use a test secret
	secret := "test-websub-secret-12345"

	// Generate what YouTube would send (SHA1 signature)
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := mac.Sum(nil)
	signature := "sha1=" + hex.EncodeToString(expectedMAC)

	// Verify it works
	if !VerifySignature(payload, signature, secret) {
		t.Error("Expected real-world YouTube-style SHA1 signature to be valid")
	}
}
