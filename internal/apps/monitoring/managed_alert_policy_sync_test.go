/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package monitoring

import (
	"strings"
	"testing"
)

func TestBuildManagedMetricAlertDescriptionAnnotation_prefersChineseTemplateDescription(t *testing.T) {
	policy := &AlertPolicy{
		Name:        "内存 0.5",
		Description: "",
		TemplateKey: "memory_usage_high",
	}
	template := &AlertPolicyTemplateSummaryDTO{
		Key:         "memory_usage_high",
		Name:        "Heap memory usage high",
		Description: "Alert when the SeaTunnel JVM heap usage ratio stays above the threshold.",
	}
	condition := resolvedMetricPolicyCondition{
		Operator:      ">",
		Threshold:     "0.05",
		WindowMinutes: 1,
	}

	description := buildManagedMetricAlertDescriptionAnnotation(policy, template, condition)
	if !strings.Contains(description, "堆内存") {
		t.Fatalf("expected chinese template description, got %s", description)
	}
	if strings.Contains(description, "SeaTunnel JVM heap usage ratio") {
		t.Fatalf("expected english fallback to be removed, got %s", description)
	}
}
