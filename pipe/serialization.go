package pipe

// Serialization is an enum-like string type for serialization formats
type Serialization string

const (
	SerializationProtobuf Serialization = "protobuf"
	SerializationJSON     Serialization = "json"
)

// String returns the string value
func (s Serialization) String() string {
	return string(s)
}

// IsValid checks if the value is one of the known constants
func (s Serialization) IsValid() bool {
	return s == SerializationProtobuf || s == SerializationJSON
}
