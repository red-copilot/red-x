package mongowire

import (
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/xdg-go/scram"
)

const (
	opMsg          = 2013
	maxMessageSize = 64 << 20
)

var requestID atomic.Uint32

type authError struct {
	code   int32
	reason string
}

func (e *authError) Error() string {
	if e.reason == "" {
		return fmt.Sprintf("mongodb authentication failed (code %d)", e.code)
	}
	return fmt.Sprintf("mongodb authentication failed (code %d): %s", e.code, e.reason)
}

// Ping verifies that a MongoDB endpoint speaks the wire protocol.
func Ping(ctx context.Context, address string) error {
	conn, err := dial(ctx, address)
	if err != nil {
		return err
	}
	defer conn.Close()
	response, err := command(ctx, conn, Document{{Key: "hello", Value: int32(1)}, {Key: "$db", Value: "admin"}})
	if err != nil {
		return err
	}
	if !docOK(response) {
		return commandError(response)
	}
	return nil
}

// ProbeUnauthenticated reports whether an unauthenticated client can list databases.
// Authentication-enabled servers reject this command with Unauthorized.
func ProbeUnauthenticated(ctx context.Context, address string) error {
	conn, err := dial(ctx, address)
	if err != nil {
		return err
	}
	defer conn.Close()
	response, err := command(ctx, conn, Document{
		{Key: "listDatabases", Value: int32(1)},
		{Key: "nameOnly", Value: true},
		{Key: "$db", Value: "admin"},
	})
	if err != nil {
		return err
	}
	if !docOK(response) {
		return commandError(response)
	}
	return nil
}

// Authenticate performs a SCRAM-SHA-256 authentication, falling back to
// SCRAM-SHA-1 when the server does not advertise SHA-256.
func Authenticate(ctx context.Context, address, username, password string) error {
	if username == "" {
		return errors.New("mongodb username is required")
	}
	mechanisms, err := supportedMechanisms(ctx, address, username)
	if err != nil {
		return err
	}
	ordered := preferredMechanisms(mechanisms)
	if len(mechanisms) == 0 {
		ordered = []string{"SCRAM-SHA-256", "SCRAM-SHA-1"}
	}
	if len(ordered) == 0 {
		return errors.New("mongodb server does not support SCRAM-SHA-256 or SCRAM-SHA-1")
	}
	var last error
	for _, mechanism := range ordered {
		if err := authenticateSCRAM(ctx, address, username, password, mechanism); err == nil {
			return nil
		} else {
			last = err
		}
	}
	return last
}

func mongoPasswordDigest(username, password string) string {
	sum := md5.Sum([]byte(username + ":mongo:" + password))
	return hex.EncodeToString(sum[:])
}

func preferredMechanisms(advertised []string) []string {
	ordered := make([]string, 0, 2)
	seen := map[string]bool{}
	for _, candidate := range []string{"SCRAM-SHA-256", "SCRAM-SHA-1"} {
		for _, mechanism := range advertised {
			if mechanism == candidate && !seen[mechanism] {
				ordered = append(ordered, mechanism)
				seen[mechanism] = true
			}
		}
	}
	return ordered
}

func supportedMechanisms(ctx context.Context, address, username string) ([]string, error) {
	conn, err := dial(ctx, address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	response, err := command(ctx, conn, Document{
		{Key: "hello", Value: int32(1)},
		{Key: "saslSupportedMechs", Value: "admin." + username},
		{Key: "$db", Value: "admin"},
	})
	if err != nil {
		return nil, err
	}
	if !docOK(response) {
		return nil, commandError(response)
	}
	return docStringSlice(response, "saslSupportedMechs"), nil
}

func authenticateSCRAM(ctx context.Context, address, username, password, mechanism string) error {
	conn, err := dial(ctx, address)
	if err != nil {
		return err
	}
	defer conn.Close()
	hello, err := command(ctx, conn, Document{{Key: "hello", Value: int32(1)}, {Key: "$db", Value: "admin"}})
	if err != nil {
		return err
	}
	if !docOK(hello) {
		return commandError(hello)
	}
	hash := scram.SHA256
	if mechanism == "SCRAM-SHA-1" {
		hash = scram.SHA1
		password = mongoPasswordDigest(username, password)
	}
	client, err := hash.NewClient(username, password, "")
	if err != nil {
		return err
	}
	conversation := client.NewConversation()
	first, err := conversation.Step("")
	if err != nil {
		return err
	}
	response, err := command(ctx, conn, Document{
		{Key: "saslStart", Value: int32(1)},
		{Key: "mechanism", Value: mechanism},
		{Key: "payload", Value: Binary{Subtype: 0, Data: []byte(first)}},
		{Key: "autoAuthorize", Value: int32(1)},
		{Key: "options", Value: Document{{Key: "skipEmptyExchange", Value: true}}},
		{Key: "$db", Value: "admin"},
	})
	if err != nil {
		return err
	}
	if !docOK(response) {
		return commandError(response)
	}
	conversationID := docInt32(response, "conversationId")
	serverFirst := docBinary(response, "payload")
	clientFinal, err := conversation.Step(string(serverFirst))
	if err != nil {
		return err
	}
	response, err = command(ctx, conn, Document{
		{Key: "saslContinue", Value: int32(1)},
		{Key: "conversationId", Value: conversationID},
		{Key: "payload", Value: Binary{Subtype: 0, Data: []byte(clientFinal)}},
		{Key: "$db", Value: "admin"},
	})
	if err != nil {
		return err
	}
	if !docOK(response) {
		return commandError(response)
	}
	serverFinal := docBinary(response, "payload")
	if len(serverFinal) > 0 {
		if _, err := conversation.Step(string(serverFinal)); err != nil {
			return err
		}
	}
	if docBool(response, "done") {
		if !conversation.Valid() {
			return errors.New("mongodb SCRAM server signature did not validate")
		}
		return nil
	}
	response, err = command(ctx, conn, Document{
		{Key: "saslContinue", Value: int32(1)},
		{Key: "conversationId", Value: conversationID},
		{Key: "payload", Value: Binary{Subtype: 0}},
		{Key: "$db", Value: "admin"},
	})
	if err != nil {
		return err
	}
	if !docOK(response) {
		return commandError(response)
	}
	if !conversation.Valid() {
		return errors.New("mongodb authentication did not complete")
	}
	return nil
}

func dial(ctx context.Context, address string) (net.Conn, error) {
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(6 * time.Second))
	return conn, nil
}

func command(ctx context.Context, conn net.Conn, doc Document) (map[string]any, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	id := requestID.Add(1)
	body, err := EncodeDocument(doc)
	if err != nil {
		return nil, err
	}
	message := make([]byte, 21+len(body))
	binary.LittleEndian.PutUint32(message[0:4], uint32(len(message)))
	binary.LittleEndian.PutUint32(message[4:8], id)
	binary.LittleEndian.PutUint32(message[8:12], 0)
	binary.LittleEndian.PutUint32(message[12:16], opMsg)
	// flags[16:20] = 0; section kind[20] = 0.
	copy(message[21:], body)
	if _, err := conn.Write(message); err != nil {
		return nil, err
	}
	var header [16]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return nil, err
	}
	length := int(binary.LittleEndian.Uint32(header[0:4]))
	if length < 21 || length > maxMessageSize {
		return nil, fmt.Errorf("invalid mongodb response length %d", length)
	}
	if binary.LittleEndian.Uint32(header[8:12]) != id {
		return nil, fmt.Errorf("mongodb responseTo %d does not match request %d", binary.LittleEndian.Uint32(header[8:12]), id)
	}
	if binary.LittleEndian.Uint32(header[12:16]) != opMsg {
		return nil, fmt.Errorf("unexpected mongodb opcode %d", binary.LittleEndian.Uint32(header[12:16]))
	}
	rest := make([]byte, length-16)
	if _, err := io.ReadFull(conn, rest); err != nil {
		return nil, err
	}
	flags := binary.LittleEndian.Uint32(rest[0:4])
	payload := rest[4:]
	if len(payload) < 1 || payload[0] != 0 {
		return nil, errors.New("unsupported mongodb OP_MSG section")
	}
	if flags&1 != 0 && len(payload) >= 5 {
		payload = payload[:len(payload)-4]
	}
	return DecodeDocument(payload[1:])
}

func commandError(doc map[string]any) error {
	return &authError{code: docInt32(doc, "code"), reason: docString(doc, "errmsg")}
}
