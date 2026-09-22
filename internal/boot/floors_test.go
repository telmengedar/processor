package boot

import (
	"strings"
	"testing"
)

const (
	envTokensFloor = "PROCESSOR_MODEL_FLOOR_TOKENS_PER_SECOND"
	envPromptFloor = "PROCESSOR_MODEL_FLOOR_PROMPT_BYTES_PER_SECOND"
)

func TestModelFloorsAreAbsentWhereTheDeploymentDeclaresNeitherOfThem(t *testing.T) {
	t.Parallel()

	floors, err := loadModelFloors(fixedLookup(validEnv(nil)))
	if err != nil {
		t.Fatalf("loadModelFloors: %v", err)
	}
	if floors.TokensPerSecond != nil || floors.PromptBytesPerSecond != nil {
		t.Fatalf("floors = %+v, want both absent so the product's own declaration stands in", floors)
	}
}

func TestModelFloorsCarryTheRatesTheDeploymentDeclared(t *testing.T) {
	t.Parallel()

	env := validEnv(map[string]string{envTokensFloor: "23.5", envPromptFloor: "7100"})

	floors, err := loadModelFloors(fixedLookup(env))
	if err != nil {
		t.Fatalf("loadModelFloors: %v", err)
	}
	if floors.TokensPerSecond == nil || *floors.TokensPerSecond != 23.5 {
		t.Fatalf("TokensPerSecond = %v, want a pointer to 23.5", floors.TokensPerSecond)
	}
	if floors.PromptBytesPerSecond == nil || *floors.PromptBytesPerSecond != 7100 {
		t.Fatalf("PromptBytesPerSecond = %v, want a pointer to 7100", floors.PromptBytesPerSecond)
	}
}

func TestAModelFloorSetButEmptyIsAStartupErrorNamingItsVariable(t *testing.T) {
	t.Parallel()

	for _, key := range []string{envTokensFloor, envPromptFloor} {
		_, err := loadModelFloors(fixedLookup(validEnv(map[string]string{key: ""})))
		if err == nil {
			t.Fatalf("%s set but empty was accepted; an emptied variable is an operator mistake, never a request for the product's declaration", key)
		}
		if !strings.Contains(err.Error(), key) {
			t.Fatalf("err = %v, want it to name %s", err, key)
		}
	}
}

func TestANonNumericModelFloorIsAStartupErrorNamingItsVariable(t *testing.T) {
	t.Parallel()

	for _, key := range []string{envTokensFloor, envPromptFloor} {
		_, err := loadModelFloors(fixedLookup(validEnv(map[string]string{key: "quickly"})))
		if err == nil {
			t.Fatalf("%s was accepted with a non-numeric value", key)
		}
		if !strings.Contains(err.Error(), key) {
			t.Fatalf("err = %v, want it to name %s", err, key)
		}
	}
}

func TestAModelFloorOfZeroOrLessIsAStartupErrorBecauseNoEndpointDeliversAtThatRate(t *testing.T) {
	t.Parallel()

	for _, key := range []string{envTokensFloor, envPromptFloor} {
		for _, value := range []string{"0", "-4"} {
			_, err := loadModelFloors(fixedLookup(validEnv(map[string]string{key: value})))
			if err == nil {
				t.Fatalf("%s = %q was accepted; a rate of zero or less asserts an endpoint that never finishes, and it would divide the affordability check by zero", key, value)
			}
			if !strings.Contains(err.Error(), key) {
				t.Fatalf("err = %v, want it to name %s", err, key)
			}
		}
	}
}
