package credentialstore

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

const protectedPartSize = 2400

type protectedRecords interface {
	read(string) ([]byte, error)
	write(string, []byte) error
	remove(string) error
	names(string) ([]string, error)
}
type refreshRecord struct {
	Schema     string `json:"schema"`
	Generation string `json:"generation"`
	Parts      int    `json:"parts"`
	Length     int    `json:"length"`
	Digest     string `json:"sha256"`
}

func partPrefix(target string) string { return target + "/parts/" }
func partTarget(target, generation string, index int) string {
	return partPrefix(target) + generation + "/" + strconv.Itoa(index)
}
func readRefreshRecord(records protectedRecords, target string) (refreshRecord, error) {
	data, err := records.read(target)
	if err != nil {
		return refreshRecord{}, err
	}
	defer clear(data)
	var r refreshRecord
	if len(data) > 1024 || json.Unmarshal(data, &r) != nil || r.Schema != "alzette.windows-refresh.v1" || len(r.Generation) != 32 || len(r.Digest) != 64 || r.Length < 16 || r.Length > 16<<10 || r.Parts != (r.Length+protectedPartSize-1)/protectedPartSize {
		return r, fmt.Errorf("%w: protected refresh record is invalid", ErrUnavailable)
	}
	if _, err = hex.DecodeString(r.Generation); err != nil {
		return r, ErrUnavailable
	}
	if _, err = hex.DecodeString(r.Digest); err != nil {
		return r, ErrUnavailable
	}
	return r, nil
}
func loadRefreshRecord(records protectedRecords, target string) (string, error) {
	r, err := readRefreshRecord(records, target)
	if err != nil {
		return "", err
	}
	value := make([]byte, 0, r.Length)
	defer func() { clear(value) }()
	for index := 0; index < r.Parts; index++ {
		part, e := records.read(partTarget(target, r.Generation, index))
		if e != nil {
			return "", fmt.Errorf("%w: protected refresh credential is incomplete", ErrUnavailable)
		}
		want := protectedPartSize
		if index == r.Parts-1 {
			want = r.Length - index*protectedPartSize
		}
		if len(part) != want {
			clear(part)
			return "", ErrUnavailable
		}
		value = append(value, part...)
		clear(part)
	}
	digest := sha256.Sum256(value)
	if hex.EncodeToString(digest[:]) != r.Digest {
		return "", fmt.Errorf("%w: protected refresh credential failed integrity validation", ErrUnavailable)
	}
	return string(value), nil
}
func saveRefreshRecord(records protectedRecords, target, value string) error {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	generation := hex.EncodeToString(nonce[:])
	digest := sha256.Sum256([]byte(value))
	r := refreshRecord{Schema: "alzette.windows-refresh.v1", Generation: generation, Parts: (len(value) + protectedPartSize - 1) / protectedPartSize, Length: len(value), Digest: hex.EncodeToString(digest[:])}
	written := []string{}
	discard := func() {
		for _, name := range written {
			_ = records.remove(name)
		}
	}
	for index := 0; index < r.Parts; index++ {
		end := (index + 1) * protectedPartSize
		if end > len(value) {
			end = len(value)
		}
		part := []byte(value[index*protectedPartSize : end])
		name := partTarget(target, generation, index)
		err := records.write(name, part)
		clear(part)
		if err != nil {
			discard()
			return err
		}
		written = append(written, name)
	}
	manifest, _ := json.Marshal(r)
	// Publish only after all pieces are protected. CredWrite replaces this one
	// record atomically; a failed rotation leaves the previous token intact.
	if err := records.write(target, manifest); err != nil {
		discard()
		return err
	}
	return pruneRefreshParts(records, target, generation)
}
func pruneRefreshParts(records protectedRecords, target, keep string) error {
	prefix := partPrefix(target)
	names, err := records.names(prefix)
	if err != nil {
		return err
	}
	for _, name := range names {
		if !strings.HasPrefix(name, prefix) {
			return ErrUnavailable
		}
		if keep != "" && strings.HasPrefix(name, prefix+keep+"/") {
			continue
		}
		if err = records.remove(name); err != nil {
			return err
		}
	}
	return nil
}
func deleteRefreshRecord(records protectedRecords, target string) error {
	if err := records.remove(target); err != nil {
		return err
	}
	return pruneRefreshParts(records, target, "")
}
