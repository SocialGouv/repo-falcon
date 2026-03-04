package graph

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
)

const (
	idPrefix = "sha256:"
)

func id(prefix string, fields ...string) (idOut string, key string) {
	key = prefix + "\n" + StableJoin(fields...)
	sum := sha256.Sum256([]byte(key))
	return idPrefix + hex.EncodeToString(sum[:]), key
}

// FileKey returns the canonical key string used for file IDs.
func FileKey(repoRelPath string) string {
	p := MustCanonRepoRelPath(repoRelPath)
	_, key := id("file:v1", p)
	return key
}

// NewFileID returns a deterministic file node id.
func NewFileID(repoRelPath string) string {
	p := MustCanonRepoRelPath(repoRelPath)
	idOut, _ := id("file:v1", p)
	return idOut
}

func PackageKey(language, name string) string {
	lang := CanonicalLanguage(language)
	name = strings.TrimSpace(name)
	_, key := id("package:v1", lang, name)
	return key
}

// NewPackageID returns a deterministic package node id.
func NewPackageID(language, name string) string {
	lang := CanonicalLanguage(language)
	name = strings.TrimSpace(name)
	idOut, _ := id("package:v1", lang, name)
	return idOut
}

func SymbolKey(language, pkg, qualifiedName, repoRelPath string, startLine, startCol, endLine, endCol int) string {
	lang := CanonicalLanguage(language)
	p := MustCanonRepoRelPath(repoRelPath)
	_, key := id(
		"symbol:v1",
		lang,
		strings.TrimSpace(pkg),
		strings.TrimSpace(qualifiedName),
		p,
		itoa(startLine), itoa(startCol), itoa(endLine), itoa(endCol),
	)
	return key
}

// NewSymbolID returns a deterministic symbol node id.
func NewSymbolID(language, pkg, qualifiedName, repoRelPath string, startLine, startCol, endLine, endCol int) string {
	lang := CanonicalLanguage(language)
	p := MustCanonRepoRelPath(repoRelPath)
	idOut, _ := id(
		"symbol:v1",
		lang,
		strings.TrimSpace(pkg),
		strings.TrimSpace(qualifiedName),
		p,
		itoa(startLine), itoa(startCol), itoa(endLine), itoa(endCol),
	)
	return idOut
}

func MessageFingerprintHex(message string) string {
	msg := strings.TrimSpace(message)
	msg = strings.Join(strings.Fields(msg), " ")
	sum := sha256.Sum256([]byte(msg))
	return hex.EncodeToString(sum[:])
}

func FindingKey(tool, ruleID, repoRelPath string, startLine, startCol int, message string) string {
	t := strings.TrimSpace(tool)
	r := strings.TrimSpace(ruleID)
	p := MustCanonRepoRelPath(repoRelPath)
	fp := MessageFingerprintHex(message)
	_, key := id("finding:v1", t, r, p, itoa(startLine), itoa(startCol), fp)
	return key
}

// NewFindingID returns a deterministic finding node id.
func NewFindingID(tool, ruleID, repoRelPath string, startLine, startCol int, message string) string {
	t := strings.TrimSpace(tool)
	r := strings.TrimSpace(ruleID)
	p := MustCanonRepoRelPath(repoRelPath)
	fp := MessageFingerprintHex(message)
	idOut, _ := id("finding:v1", t, r, p, itoa(startLine), itoa(startCol), fp)
	return idOut
}

func EdgeKey(srcID, dstID string, edgeType EdgeType, attrsCanonical string) string {
	attrs := strings.TrimSpace(attrsCanonical)
	_, key := id("edge:v1", string(edgeType), strings.TrimSpace(srcID), strings.TrimSpace(dstID), attrs)
	return key
}

// NewEdgeID returns a deterministic edge id.
func NewEdgeID(srcID, dstID string, edgeType EdgeType, attrsCanonical string) string {
	attrs := strings.TrimSpace(attrsCanonical)
	idOut, _ := id("edge:v1", string(edgeType), strings.TrimSpace(srcID), strings.TrimSpace(dstID), attrs)
	return idOut
}

func itoa(v int) string {
	return strconv.FormatInt(int64(v), 10)
}
