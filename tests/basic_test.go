/*
 * Copyright 2021 NMPolicy Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *	  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tests

import (
	"testing"
	"time"

	"sigs.k8s.io/yaml"

	assert "github.com/stretchr/testify/require"

	"github.com/nmstate/nmpolicy/nmpolicy"
	"github.com/nmstate/nmpolicy/nmpolicy/types"
)

func TestBasicPolicy(t *testing.T) {
	t.Run("Basic policy", func(t *testing.T) {
		testEmptyPolicy(t)
		testPolicyWithOnlyDesiredState(t)
		testPolicyWithCachedCaptureAndDesiredStateWithoutRef(t)
		testPolicyWithFilterCaptureAndDesiredStateWithoutRef(t)
		testGenerateUniqueTimestamps(t)
	})
}

func testEmptyPolicy(t *testing.T) {
	t.Run("is empty", func(t *testing.T) {
		s, err := nmpolicy.GenerateState(types.PolicySpec{}, nil, types.NoCache())

		assert.NoError(t, err)

		expectedEmptyState := types.GeneratedState{MetaInfo: types.MetaInfo{Version: "0"}}
		assert.NotEqual(t, time.Time{}, s.MetaInfo.TimeStamp)
		assert.Equal(t, expectedEmptyState, resetTimeStamp(s))
	})
}

func testPolicyWithOnlyDesiredState(t *testing.T) {
	// When a basic input with only the desired state is provided,
	// the policy just passes it as is to the output with no modifications.
	t.Run("with only desired state", func(t *testing.T) {
		stateData := []byte(`this is not a legal yaml format!
`)
		policySpec := types.PolicySpec{
			DesiredState: stateData,
		}

		s, err := nmpolicy.GenerateState(policySpec, nil, types.NoCache())

		assert.NoError(t, err)
		expectedState := types.GeneratedState{
			DesiredState: stateData,
			MetaInfo:     types.MetaInfo{Version: "0"},
		}
		assert.Equal(t, expectedState, resetTimeStamp(s))
	})
}

func testPolicyWithCachedCaptureAndDesiredStateWithoutRef(t *testing.T) {
	t.Run("with all captures cached and desired state that has no ref", func(t *testing.T) {
		stateData := []byte(`this is not a legal yaml format!
`)
		const capID0 = "cap0"
		policySpec := types.PolicySpec{
			Capture: map[string]string{
				capID0: "my expression",
			},
			DesiredState: stateData,
		}

		cacheState := types.CachedState{
			Capture: map[string]types.CaptureState{capID0: {State: []byte("some captured state")}},
		}
		s, err := nmpolicy.GenerateState(
			policySpec,
			nil,
			cacheState)

		assert.NoError(t, err)
		expectedState := types.GeneratedState{
			Cache:        cacheState,
			DesiredState: stateData,
			MetaInfo:     types.MetaInfo{Version: "0"},
		}
		assert.Equal(t, expectedState, resetTimeStamp(s))
	})
}

func testPolicyWithFilterCaptureAndDesiredStateWithoutRef(t *testing.T) {
	t.Run("with a eqfilter capture expression and desired state that has no ref", func(t *testing.T) {
		stateData := []byte(`routes:
  running:
  - destination: 0.0.0.0/0
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
  - destination: 1.1.1.0/24
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
`)
		const capID0 = "cap0"
		policySpec := types.PolicySpec{
			Capture: map[string]string{
				capID0: `routes.running.destination=="0.0.0.0/0"`,
			},
			DesiredState: stateData,
		}
		obtained, err := nmpolicy.GenerateState(
			policySpec,
			stateData,
			types.CachedState{})
		assert.NoError(t, err)

		expected := types.GeneratedState{
			MetaInfo: types.MetaInfo{
				Version: "0",
			},
			DesiredState: stateData,
			Cache: types.CachedState{
				Capture: map[string]types.CaptureState{
					capID0: {
						State: []byte(`routes:
  running:
  - destination: 0.0.0.0/0
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
`),
					},
				},
			},
		}

		obtained = resetTimeStamp(obtained)

		obtained, err = formatYAMLs(obtained)
		assert.NoError(t, err)

		expected, err = formatYAMLs(expected)
		assert.NoError(t, err)

		assert.Equal(t, expected, obtained)
	})
}

func testGenerateUniqueTimestamps(t *testing.T) {
	t.Run("with eq filter and no desired state should set unique timestamps", func(t *testing.T) {
		stateData := []byte(`routes:
  running:
  - destination: 0.0.0.0/0
    next-hop-address: 192.168.100.1
    next-hop-interface: eth1
    table-id: 254
`)

		const capID0 = "cap0"
		const capID1 = "cap1"
		policySpec := types.PolicySpec{
			Capture: map[string]string{
				capID0: `routes.running.destination=="0.0.0.0/0"`,
				capID1: `routes.running.destination=="1.1.1.1/0"`,
			},
			DesiredState: stateData,
		}
		cacheState := types.CachedState{
			Capture: map[string]types.CaptureState{
				capID1: {
					State: []byte("{}"),
					MetaInfo: types.MetaInfo{
						Version:   "333",
						TimeStamp: time.Now(),
					},
				},
			},
		}
		beforeGenerate := time.Now()
		obtained, err := nmpolicy.GenerateState(
			policySpec,
			stateData,
			cacheState)
		assert.NoError(t, err)
		assert.Equal(t, obtained.MetaInfo.TimeStamp, obtained.Cache.Capture[capID0].MetaInfo.TimeStamp)
		assert.Equal(t, cacheState.Capture[capID1].MetaInfo, obtained.Cache.Capture[capID1].MetaInfo)
		assert.Greater(t, obtained.MetaInfo.TimeStamp.Sub(beforeGenerate), time.Duration(0))
		assert.Greater(t, obtained.Cache.Capture[capID0].MetaInfo.TimeStamp.Sub(beforeGenerate), time.Duration(0))
	})
}

func resetTimeStamp(generatedState types.GeneratedState) types.GeneratedState {
	generatedState.MetaInfo.TimeStamp = time.Time{}
	for captureID, captureState := range generatedState.Cache.Capture {
		captureState.MetaInfo.TimeStamp = time.Time{}
		generatedState.Cache.Capture[captureID] = captureState
	}
	return generatedState
}

func formatYAMLs(generatedState types.GeneratedState) (types.GeneratedState, error) {
	for captureID, captureState := range generatedState.Cache.Capture {
		formatedYAML, err := formatYAML(captureState.State)
		if err != nil {
			return types.GeneratedState{}, err
		}
		captureState.State = formatedYAML
		generatedState.Cache.Capture[captureID] = captureState
	}
	return generatedState, nil
}

func formatYAML(unformatedYAML []byte) ([]byte, error) {
	unmarshaled := map[string]interface{}{}

	err := yaml.Unmarshal(unformatedYAML, &unmarshaled)
	if err != nil {
		return nil, err
	}

	marshaled, err := yaml.Marshal(unmarshaled)
	if err != nil {
		return nil, err
	}
	return marshaled, nil
}
