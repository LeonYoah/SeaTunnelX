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
import org.apache.seatunnel.shade.com.typesafe.config.ConfigResolveOptions;
import org.apache.seatunnel.shade.com.typesafe.config.ConfigSyntax;
import org.apache.seatunnel.shade.com.typesafe.config.impl.Parseable;
import org.apache.seatunnel.shade.org.apache.commons.lang3.StringUtils;

import org.apache.seatunnel.api.configuration.ReadonlyConfig;
import org.apache.seatunnel.core.starter.utils.ConfigBuilder;
import org.apache.seatunnel.engine.core.parse.ConfigParserUtil;
import org.apache.seatunnel.tools.proxy.model.DatasetDag;
import org.apache.seatunnel.tools.proxy.model.JobConfigContext;
import org.apache.seatunnel.tools.proxy.model.NodeKind;
import org.apache.seatunnel.tools.proxy.model.ProxyEdge;
import org.apache.seatunnel.tools.proxy.model.ProxyNode;

import java.util.ArrayList;
import java.util.Collections;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Properties;

import static org.apache.seatunnel.api.options.ConnectorCommonOptions.PLUGIN_INPUT;
import static org.apache.seatunnel.api.options.ConnectorCommonOptions.PLUGIN_NAME;
import static org.apache.seatunnel.api.options.ConnectorCommonOptions.PLUGIN_OUTPUT;
import static org.apache.seatunnel.api.table.factory.FactoryUtil.DEFAULT_ID;

public class JobConfigSupportService {

    public JobConfigContext parseJobContext(Map<String, Object> request) {
        try {
            Config jobConfig = loadJobConfig(request);
            List<Config> sources = getConfigList(jobConfig, "source");
            List<Config> transforms = getConfigList(jobConfig, "transform");
            List<Config> sinks = getConfigList(jobConfig, "sink");

            ConfigParserUtil.checkGraph(sources, transforms, sinks);

            boolean simpleGraph = isSimpleGraph(sources, transforms, sinks);
            List<String> warnings = new ArrayList<>();
            DatasetDag graph = buildGraph(sources, transforms, sinks, simpleGraph, warnings);
            return new JobConfigContext(
                    jobConfig, sources, transforms, sinks, simpleGraph, warnings, graph);
        } catch (ProxyException e) {
            throw e;
        } catch (Exception e) {
            throw new ProxyException(400, "Config dag inspection failed: " + e.getMessage(), e);
        }
    }

    public String nodeId(NodeKind kind, int index) {
        return kind.name().toLowerCase() + "-" + index;
    }

    public String getOutputDataset(ReadonlyConfig config) {
        return config.getOptional(PLUGIN_OUTPUT).orElse(DEFAULT_ID);
    }

    public List<String> getInputDatasets(ReadonlyConfig config) {
        return config.getOptional(PLUGIN_INPUT).orElse(Collections.singletonList(DEFAULT_ID));
    }

    private DatasetDag buildGraph(
            List<Config> sources,
            List<Config> transforms,
            List<Config> sinks,
            boolean simpleGraph,
            List<String> warnings) {
        List<ProxyNode> nodes = new ArrayList<>();
        List<ProxyEdge> edges = new ArrayList<>();
        Map<String, String> producers = new LinkedHashMap<>();

        for (int i = 0; i < sources.size(); i++) {
            ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(sources.get(i));
            String nodeId = nodeId(NodeKind.SOURCE, i);
            String output = getOutputDataset(readonlyConfig);
            nodes.add(
                    new ProxyNode(
                            nodeId,
                            NodeKind.SOURCE,
                            readonlyConfig.get(PLUGIN_NAME),
                            i,
                            Collections.emptyList(),
                            output));
            producers.put(output, nodeId);
        }

        for (int i = 0; i < transforms.size(); i++) {
            ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(transforms.get(i));
            String nodeId = nodeId(NodeKind.TRANSFORM, i);
            List<String> inputs = getInputDatasets(readonlyConfig);
            String output = getOutputDataset(readonlyConfig);
            nodes.add(
                    new ProxyNode(
                            nodeId,
                            NodeKind.TRANSFORM,
                            readonlyConfig.get(PLUGIN_NAME),
                            i,
                            inputs,
                            output));
            producers.put(output, nodeId);
        }

        for (int i = 0; i < sinks.size(); i++) {
            ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(sinks.get(i));
            String nodeId = nodeId(NodeKind.SINK, i);
            List<String> inputs = getInputDatasets(readonlyConfig);
            nodes.add(
                    new ProxyNode(
                            nodeId,
                            NodeKind.SINK,
                            readonlyConfig.get(PLUGIN_NAME),
                            i,
                            inputs,
                            null));
        }

        if (simpleGraph) {
            if (transforms.isEmpty()) {
                addSimpleEdge(
                        sources.get(0),
                        nodeId(NodeKind.SOURCE, 0),
                        sinks.get(0),
                        nodeId(NodeKind.SINK, 0),
                        warnings,
                        edges);
            } else {
                addSimpleEdge(
                        sources.get(0),
                        nodeId(NodeKind.SOURCE, 0),
                        transforms.get(0),
                        nodeId(NodeKind.TRANSFORM, 0),
                        warnings,
                        edges);
                addSimpleEdge(
                        transforms.get(0),
                        nodeId(NodeKind.TRANSFORM, 0),
                        sinks.get(0),
                        nodeId(NodeKind.SINK, 0),
                        warnings,
                        edges);
            }
            return new DatasetDag(nodes, edges);
        }

        for (int i = 0; i < transforms.size(); i++) {
            ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(transforms.get(i));
            String nodeId = nodeId(NodeKind.TRANSFORM, i);
            for (String input : getInputDatasets(readonlyConfig)) {
                edges.add(new ProxyEdge(input, producers.get(input), nodeId));
            }
        }

        for (int i = 0; i < sinks.size(); i++) {
            ReadonlyConfig readonlyConfig = ReadonlyConfig.fromConfig(sinks.get(i));
            String nodeId = nodeId(NodeKind.SINK, i);
            for (String input : getInputDatasets(readonlyConfig)) {
                edges.add(new ProxyEdge(input, producers.get(input), nodeId));
            }
        }

        return new DatasetDag(nodes, edges);
    }

    private void addSimpleEdge(
            Config left,
            String leftNodeId,
            Config right,
            String rightNodeId,
            List<String> warnings,
            List<ProxyEdge> edges) {
        ReadonlyConfig leftConfig = ReadonlyConfig.fromConfig(left);
        ReadonlyConfig rightConfig = ReadonlyConfig.fromConfig(right);
        String output = getOutputDataset(leftConfig);
        String input = getInputDatasets(rightConfig).get(0);
        String dataset =
                resolveSimpleDataset(
                        output,
                        input,
                        leftConfig.get(PLUGIN_NAME),
                        rightConfig.get(PLUGIN_NAME),
                        warnings);
        edges.add(new ProxyEdge(dataset, leftNodeId, rightNodeId));
    }

    private String resolveSimpleDataset(
            String output,
            String input,
            String leftPluginName,
            String rightPluginName,
            List<String> warnings) {
        if (StringUtils.equals(output, input)) {
            return output;
        }
        if (DEFAULT_ID.equals(output) && !DEFAULT_ID.equals(input)) {
            warnings.add(
                    String.format(
                            "Simple graph compatibility linked %s to %s by sink/transform input '%s'.",
                            leftPluginName, rightPluginName, input));
            return input;
        }
        if (!DEFAULT_ID.equals(output) && DEFAULT_ID.equals(input)) {
            warnings.add(
                    String.format(
                            "Simple graph compatibility linked %s to %s by source/transform output '%s'.",
                            leftPluginName, rightPluginName, output));
            return output;
        }
        warnings.add(
                String.format(
                        "Simple graph compatibility linked %s(%s) to %s(%s) even though dataset ids differ.",
                        leftPluginName, output, rightPluginName, input));
        return output;
    }

    private Config loadJobConfig(Map<String, Object> request) {
        String content = ProxyRequestUtils.getOptionalString(request, "content");
        String contentFormat = ProxyRequestUtils.getOptionalString(request, "contentFormat");
        String filePath = ProxyRequestUtils.getOptionalString(request, "filePath");
        List<String> variables = ProxyRequestUtils.getStringList(request, "variables");

        if (StringUtils.isNotBlank(content) && StringUtils.isNotBlank(filePath)) {
            throw new ProxyException(400, "Only one of 'content' or 'filePath' can be specified");
        }
        if (StringUtils.isBlank(content) && StringUtils.isBlank(filePath)) {
            throw new ProxyException(400, "Either 'content' or 'filePath' must be provided");
        }

        if (StringUtils.isNotBlank(filePath)) {
            return variables.isEmpty()
                    ? ConfigBuilder.of(filePath)
                    : ConfigBuilder.of(filePath, variables);
        }
        return parseConfigContent(content, variables, contentFormat);
    }

    private Config parseConfigContent(
            String content, List<String> variables, String contentFormat) {
        ConfigResolveOptions resolveOptions =
                ConfigResolveOptions.defaults().setAllowUnresolved(true);
        ConfigParseOptions parseOptions = buildParseOptions(contentFormat);
        Config config = ConfigFactory.parseString(content, parseOptions).resolve(resolveOptions);

        if (!variables.isEmpty()) {
            Properties properties = new Properties();
            for (String variable : variables) {
                if (variable == null) {
                    continue;
                }
                String[] pair = variable.split("=", 2);
                if (pair.length != 2 || StringUtils.isBlank(pair[0])) {
                    throw new ProxyException(
                            400, "Invalid variable, expected key=value: " + variable);
                }
                properties.setProperty(pair[0], pair[1]);
            }
            Config variableConfig =
                    Parseable.newProperties(
                                    properties,
                                    ConfigParseOptions.defaults()
                                            .setOriginDescription("proxy variables"))
                            .parse()
                            .toConfig();
            config = config.resolveWith(variableConfig, resolveOptions);
        }

        return config.resolveWith(ConfigFactory.systemProperties(), resolveOptions);
    }

    private ConfigParseOptions buildParseOptions(String contentFormat) {
        if (StringUtils.isBlank(contentFormat) || "hocon".equalsIgnoreCase(contentFormat)) {
            return ConfigParseOptions.defaults();
        }
        if ("json".equalsIgnoreCase(contentFormat)) {
            return ConfigParseOptions.defaults().setSyntax(ConfigSyntax.JSON);
        }
        throw new ProxyException(
                400, "Unsupported contentFormat: " + contentFormat + ", expected hocon or json");
    }

    private List<Config> getConfigList(Config jobConfig, String path) {
        if (!jobConfig.hasPath(path)) {
            return Collections.emptyList();
        }
        return new ArrayList<>(jobConfig.getConfigList(path));
    }

    private boolean isSimpleGraph(
            List<Config> sources, List<Config> transforms, List<Config> sinks) {
        return sources.size() == 1
                && sinks.size() == 1
                && (transforms.isEmpty() || transforms.size() == 1);
    }
}
