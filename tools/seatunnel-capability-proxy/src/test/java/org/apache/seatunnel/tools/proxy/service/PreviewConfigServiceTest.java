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

import org.apache.seatunnel.shade.com.typesafe.config.Config;
import org.apache.seatunnel.shade.com.typesafe.config.ConfigFactory;
import org.apache.seatunnel.shade.com.typesafe.config.ConfigParseOptions;
import org.apache.seatunnel.shade.com.typesafe.config.ConfigSyntax;

import org.apache.seatunnel.tools.proxy.model.DagParseResult;

import org.junit.jupiter.api.Assertions;
import org.junit.jupiter.api.Test;

import java.util.LinkedHashMap;
import java.util.Map;

public class PreviewConfigServiceTest {

    private final PreviewConfigService previewConfigService = new PreviewConfigService();
    private final ConfigResourceService configResourceService = new ConfigResourceService();

    @Test
    public void testDeriveSourcePreviewFromSimpleGraph() {
        Map<String, Object> request = new LinkedHashMap<>();
        request.put(
                "content",
                "env {\n"
                        + "  parallelism = 2\n"
                        + "}\n"
                        + "source {\n"
                        + "  FakeSource {\n"
                        + "    result_table_name = \"orders\"\n"
                        + "  }\n"
                        + "}\n"
                        + "sink {\n"
                        + "  Console {\n"
                        + "  }\n"
                        + "}\n");
        request.put("contentFormat", "hocon");
        request.put("httpSink", httpSink("https://proxy.example.com/source-preview"));

        Map<String, Object> response = previewConfigService.deriveSourcePreview(request);
        Assertions.assertEquals(Boolean.TRUE, response.get("ok"));

        String content = String.valueOf(response.get("content"));
        Assertions.assertTrue(content.contains("\"plugin_name\":\"Metadata\""));
        Assertions.assertTrue(content.contains("\"plugin_name\":\"Http\""));
        Assertions.assertTrue(content.contains("__st_preview_source_0"));

        DagParseResult dag = configResourceService.inspectDag(contentRequest(content));
        Assertions.assertTrue(dag.isOk());
        Assertions.assertEquals(1, dag.getSourceCount());
        Assertions.assertEquals(1, dag.getSinkCount());
        Assertions.assertEquals(1, dag.getTransformCount());
    }

    @Test
    public void testDeriveTransformPreviewFromMultiSourceGraph() {
        Map<String, Object> request = new LinkedHashMap<>();
        request.put(
                "content",
                "source {\n"
                        + "  SourceA {\n"
                        + "    plugin_output = \"a\"\n"
                        + "  }\n"
                        + "  SourceB {\n"
                        + "    plugin_output = \"b\"\n"
                        + "  }\n"
                        + "}\n"
                        + "transform {\n"
                        + "  Joiner {\n"
                        + "    plugin_input = [\"a\", \"b\"]\n"
                        + "    plugin_output = \"joined\"\n"
                        + "  }\n"
                        + "  Cleaner {\n"
                        + "    plugin_input = [\"joined\"]\n"
                        + "    plugin_output = \"cleaned\"\n"
                        + "  }\n"
                        + "}\n"
                        + "sink {\n"
                        + "  Console {\n"
                        + "    plugin_input = [\"cleaned\"]\n"
                        + "  }\n"
                        + "}\n");
        request.put("contentFormat", "hocon");
        request.put("transformIndex", 0);
        request.put("httpSink", httpSink("https://proxy.example.com/transform-preview"));
        request.put("envOverrides", singletonMap("job.mode", "BATCH"));

        Map<String, Object> response = previewConfigService.deriveTransformPreview(request);
        Assertions.assertEquals(Boolean.TRUE, response.get("ok"));

        String content = String.valueOf(response.get("content"));
        Config derivedConfig =
                ConfigFactory.parseString(
                        content, ConfigParseOptions.defaults().setSyntax(ConfigSyntax.JSON));
        Assertions.assertEquals(2, derivedConfig.getConfigList("source").size());
        Assertions.assertEquals(2, derivedConfig.getConfigList("transform").size());
        Assertions.assertEquals(
                "Joiner", derivedConfig.getConfigList("transform").get(0).getString("plugin_name"));
        Assertions.assertEquals(
                "Metadata",
                derivedConfig.getConfigList("transform").get(1).getString("plugin_name"));
        Assertions.assertEquals(
                "Http", derivedConfig.getConfigList("sink").get(0).getString("plugin_name"));
        Assertions.assertFalse(content.contains("\"plugin_name\" : \"Cleaner\""));

        DagParseResult dag = configResourceService.inspectDag(contentRequest(content));
        Assertions.assertTrue(dag.isOk());
        Assertions.assertEquals(2, dag.getSourceCount());
        Assertions.assertEquals(2, dag.getTransformCount());
        Assertions.assertEquals(1, dag.getSinkCount());
    }

    @Test
    public void testDeriveSourcePreviewAsHocon() {
        Map<String, Object> request = new LinkedHashMap<>();
        request.put(
                "content",
                "source {\n"
                        + "  FakeSource {\n"
                        + "    plugin_output = \"src\"\n"
                        + "  }\n"
                        + "}\n"
                        + "sink {\n"
                        + "  Console {\n"
                        + "    plugin_input = [\"src\"]\n"
                        + "  }\n"
                        + "}\n");
        request.put("contentFormat", "hocon");
        request.put("httpSink", httpSink("https://proxy.example.com/source-preview"));
        request.put("outputFormat", "hocon");

        Map<String, Object> response = previewConfigService.deriveSourcePreview(request);

        Assertions.assertEquals("hocon", response.get("contentFormat"));
        String content = String.valueOf(response.get("content"));
        Assertions.assertTrue(content.contains("Metadata {"));
        Assertions.assertTrue(content.contains("Http {"));

        DagParseResult dag = configResourceService.inspectDag(contentRequest(content, "hocon"));
        Assertions.assertTrue(dag.isOk());
        Assertions.assertEquals(1, dag.getSourceCount());
        Assertions.assertEquals(1, dag.getTransformCount());
        Assertions.assertEquals(1, dag.getSinkCount());
    }

    private Map<String, Object> httpSink(String url) {
        Map<String, Object> sink = new LinkedHashMap<>();
        sink.put("url", url);
        sink.put("array_mode", false);
        return sink;
    }

    private Map<String, Object> singletonMap(String key, Object value) {
        Map<String, Object> map = new LinkedHashMap<>();
        map.put(key, value);
        return map;
    }

    private Map<String, Object> contentRequest(String content) {
        return contentRequest(content, "json");
    }

    private Map<String, Object> contentRequest(String content, String contentFormat) {
        Map<String, Object> request = new LinkedHashMap<>();
        request.put("content", content);
        request.put("contentFormat", contentFormat);
        return request;
    }
}
