package protocol

import "testing"

func TestEncodeDecodeEnvelope(t *testing.T) {
	t.Parallel()

	data, err := EncodeEnvelope(TypePing, 1, map[string]string{"foo": "bar"})
	if err != nil {
		t.Fatalf("EncodeEnvelope failed: %v", err)
	}

	env, err := DecodeEnvelope(data)
	if err != nil {
		t.Fatalf("DecodeEnvelope failed: %v", err)
	}

	if env.Type != TypePing || env.Seq != 1 {
		t.Errorf("unexpected envelope values: type=%s seq=%d", env.Type, env.Seq)
	}
}
