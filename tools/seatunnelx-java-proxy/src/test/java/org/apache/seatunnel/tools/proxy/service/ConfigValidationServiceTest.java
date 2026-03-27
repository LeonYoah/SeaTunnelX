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

package org.apache.seatunnel.tools.proxy.service;

import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

class ConfigValidationServiceTest {

    @BeforeAll
    static void setUpSeatunnelHome() {
        System.setProperty("SEATUNNEL_HOME", "/opt/seatunnel-2.3.13-new");
    }

    private final ConfigValidationService service = new ConfigValidationService();

    @Test
    void testConnectionSkippedDoesNotPass() {
        Map<String, Object> request = new LinkedHashMap<>();
        request.put(
                "content",
                String.join(
                        "\n",
                        "env {",
                        "  job.mode = \"batch\"",
                        "}",
                        "source {",
                        "  Http {",
                        "    plugin_output = \"src\"",
                        "    url = \"http://127.0.0.1:8080\"",
                        "  }",
                        "}",
                        "sink {",
                        "  Console {",
                        "    plugin_input = [\"src\"]",
                        "  }",
                        "}"));
        request.put("contentFormat", "hocon");
        request.put("testConnection", true);

        Map<String, Object> result = service.validate(request);

        assertFalse((Boolean) result.get("ok"));
        assertFalse((Boolean) result.get("valid"));
        @SuppressWarnings("unchecked")
        List<String> warnings = (List<String>) result.get("warnings");
        assertTrue(warnings.stream().anyMatch(item -> item.contains("未完成连接测试")));
    }

    @Test
    void invalidConnectorShouldFailFast() {
        Map<String, Object> request = new LinkedHashMap<>();
        request.put(
                "content",
                String.join(
                        "\n",
                        "env {",
                        "  job.mode = \"batch\"",
                        "}",
                        "source {",
                        "  JDBC2 {",
                        "    plugin_output = \"src\"",
                        "  }",
                        "}",
                        "sink {",
                        "  Console {",
                        "    plugin_input = [\"src\"]",
                        "  }",
                        "}"));
        request.put("contentFormat", "hocon");

        try {
            service.validate(request);
        } catch (ProxyException e) {
            assertEquals(400, e.getStatusCode());
            assertTrue(e.getMessage().contains("official tablePath resolution failed"));
            return;
        }
        throw new AssertionError("expected ProxyException");
    }
}
