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

import org.apache.seatunnel.tools.proxy.model.WebUiDagPreviewResult;
import org.apache.seatunnel.tools.proxy.model.WebUiDagVertexInfo;

import org.junit.jupiter.api.BeforeAll;
import org.junit.jupiter.api.Test;

import java.util.Collections;
import java.util.List;
import java.util.Map;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertIterableEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

class WebUiDagPreviewServiceTest {

    @BeforeAll
    static void setUpSeatunnelHome() {
        System.setProperty("SEATUNNEL_HOME", "/opt/seatunnel-2.3.13-new");
    }

    private final WebUiDagPreviewService service = new WebUiDagPreviewService();

    @Test
    void previewBuildsWebUiCompatibleDag() {
        WebUiDagPreviewResult result =
                preview(
                        conf(
                                "env {",
                                "  job.mode = \"batch\"",
                                "}",
                                "",
                                "source {",
                                "  Jdbc {",
                                "    plugin_output = \"users_src\"",
                                "    url = \"jdbc:mysql://127.0.0.1:3307/seatunnel_demo\"",
                                "    username = \"root\"",
                                "    password = \"seatunnel\"",
                                "    driver = \"com.mysql.cj.jdbc.Driver\"",
                                "    table_path = \"seatunnel_demo.users\"",
                                "  }",
                                "}",
                                "",
                                "sink {",
                                "  Console {",
                                "    plugin_input = [\"users_src\"]",
                                "  }",
                                "}"));

        assertEquals("preview", result.getJobId());
        assertEquals("Config Preview", result.getJobName());
        assertEquals("CREATED", result.getJobStatus());
        assertNotNull(result.getJobDag());
        assertEquals(1, result.getJobDag().getPipelineEdges().size());
        assertEquals(2, result.getJobDag().getVertexInfoMap().size());
        assertEquals("Source[0]-Jdbc", vertex(result, 1).getConnectorType());
        assertEquals("Sink[0]-Console", vertex(result, 2).getConnectorType());
        assertIterableEquals(
                Collections.singletonList("seatunnel_demo.users"),
                vertex(result, 1).getTablePaths());
        assertIterableEquals(
                Collections.singletonList("seatunnel_demo.users"),
                vertex(result, 2).getTablePaths());
        assertEquals(1, result.getJobDag().getPipelineEdges().get(0).size());
        assertFalse(result.getMetrics().isEmpty());
    }

    @Test
    void previewFailsFastWhenOfficialConnectorInitializationFails() {
        try {
            preview(
                    conf(
                            "env {",
                            "  job.mode = \"batch\"",
                            "}",
                            "",
                            "source {",
                            "  Icberg {",
                            "    plugin_output = \"orders\"",
                            "  }",
                            "}",
                            "",
                            "sink {",
                            "  Console {",
                            "    plugin_input = [\"orders\"]",
                            "  }",
                            "}"));
        } catch (ProxyException e) {
            assertTrue(e.getMessage().contains("official tablePath resolution failed"));
            return;
        }
        throw new AssertionError("expected ProxyException");
    }

    @Test
    void previewUsesOfficialJdbcSingleTableAndSinkPlaceholder() {
        WebUiDagPreviewResult result =
                preview(
                        conf(
                                "env {",
                                "  job.mode = \"batch\"",
                                "}",
                                "",
                                "source {",
                                "  Jdbc {",
                                "    plugin_output = \"users_src\"",
                                "    url = \"jdbc:mysql://127.0.0.1:3307/seatunnel_demo\"",
                                "    username = \"root\"",
                                "    password = \"seatunnel\"",
                                "    driver = \"com.mysql.cj.jdbc.Driver\"",
                                "    table_path = \"seatunnel_demo.users\"",
                                "  }",
                                "}",
                                "",
                                "sink {",
                                "  Jdbc {",
                                "    plugin_input = [\"users_src\"]",
                                "    url = \"jdbc:mysql://127.0.0.1:3307/demo2\"",
                                "    username = \"root\"",
                                "    password = \"seatunnel\"",
                                "    driver = \"com.mysql.cj.jdbc.Driver\"",
                                "    database = \"demo2\"",
                                "    table = \"${table_name}\"",
                                "    generate_sink_sql = true",
                                "  }",
                                "}"));

        assertIterableEquals(
                Collections.singletonList("seatunnel_demo.users"),
                vertex(result, 1).getTablePaths());
        assertIterableEquals(
                Collections.singletonList("demo2.users"), vertex(result, 2).getTablePaths());
    }

    @Test
    void previewUsesOfficialJdbcMultiTableAndSinkPlaceholder() {
        WebUiDagPreviewResult result =
                preview(
                        conf(
                                "env {",
                                "  job.mode = \"batch\"",
                                "}",
                                "",
                                "source {",
                                "  Jdbc {",
                                "    plugin_output = \"mysql_src\"",
                                "    url = \"jdbc:mysql://127.0.0.1:3307/seatunnel_demo\"",
                                "    username = \"root\"",
                                "    password = \"seatunnel\"",
                                "    driver = \"com.mysql.cj.jdbc.Driver\"",
                                "    table_list = [",
                                "      { table_path = \"seatunnel_demo.users\" },",
                                "      { table_path = \"seatunnel_demo.orders\" }",
                                "    ]",
                                "  }",
                                "}",
                                "",
                                "sink {",
                                "  Jdbc {",
                                "    plugin_input = [\"mysql_src\"]",
                                "    url = \"jdbc:mysql://127.0.0.1:3307/demo2\"",
                                "    username = \"root\"",
                                "    password = \"seatunnel\"",
                                "    driver = \"com.mysql.cj.jdbc.Driver\"",
                                "    database = \"demo2\"",
                                "    table = \"archive_${table_name}\"",
                                "    generate_sink_sql = true",
                                "  }",
                                "}"));

        assertIterableEquals(
                List.of("seatunnel_demo.users", "seatunnel_demo.orders"),
                vertex(result, 1).getTablePaths());
        assertIterableEquals(
                List.of("demo2.archive_users", "demo2.archive_orders"),
                vertex(result, 2).getTablePaths());
    }

    @Test
    void previewSupportsMultiSourceAndMultiSinkPaths() {
        WebUiDagPreviewResult result =
                preview(
                        conf(
                                "env {",
                                "  job.mode = \"batch\"",
                                "}",
                                "",
                                "source {",
                                "  Jdbc {",
                                "    plugin_output = \"users_src\"",
                                "    url = \"jdbc:mysql://127.0.0.1:3307/seatunnel_demo\"",
                                "    username = \"root\"",
                                "    password = \"seatunnel\"",
                                "    driver = \"com.mysql.cj.jdbc.Driver\"",
                                "    table_path = \"seatunnel_demo.users\"",
                                "  }",
                                "",
                                "  Jdbc {",
                                "    plugin_output = \"orders_src\"",
                                "    url = \"jdbc:mysql://127.0.0.1:3307/seatunnel_demo\"",
                                "    username = \"root\"",
                                "    password = \"seatunnel\"",
                                "    driver = \"com.mysql.cj.jdbc.Driver\"",
                                "    table_path = \"seatunnel_demo.orders\"",
                                "  }",
                                "}",
                                "",
                                "sink {",
                                "  Console {",
                                "    plugin_input = [\"users_src\", \"orders_src\"]",
                                "  }",
                                "",
                                "  Jdbc {",
                                "    plugin_input = [\"users_src\"]",
                                "    url = \"jdbc:mysql://127.0.0.1:3307/demo2\"",
                                "    username = \"root\"",
                                "    password = \"seatunnel\"",
                                "    driver = \"com.mysql.cj.jdbc.Driver\"",
                                "    database = \"demo2\"",
                                "    table = \"archive_users\"",
                                "    generate_sink_sql = true",
                                "  }",
                                "}"));

        assertIterableEquals(
                Collections.singletonList("seatunnel_demo.users"),
                vertexByConnector(result, "Source[0]-Jdbc").getTablePaths());
        assertIterableEquals(
                Collections.singletonList("seatunnel_demo.orders"),
                vertexByConnector(result, "Source[1]-Jdbc").getTablePaths());
        assertIterableEquals(
                List.of("seatunnel_demo.users", "seatunnel_demo.orders"),
                vertexByConnector(result, "Sink[0]-Console").getTablePaths());
        assertIterableEquals(
                Collections.singletonList("demo2.archive_users"),
                vertexByConnector(result, "Sink[1]-Jdbc").getTablePaths());
        assertEquals(4, result.getJobDag().getVertexInfoMap().size());
    }

    private WebUiDagPreviewResult preview(String content) {
        return service.preview(Map.of("content", content, "contentFormat", "hocon"));
    }

    private String conf(String... lines) {
        return String.join("\n", lines);
    }

    private WebUiDagVertexInfo vertex(WebUiDagPreviewResult result, int vertexId) {
        return result.getJobDag().getVertexInfoMap().get(vertexId);
    }

    private WebUiDagVertexInfo vertexByConnector(
            WebUiDagPreviewResult result, String connectorType) {
        return result.getJobDag().getVertexInfoMap().values().stream()
                .filter(vertex -> connectorType.equals(vertex.getConnectorType()))
                .findFirst()
                .orElseThrow(() -> new AssertionError("missing connector: " + connectorType));
    }
}
