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

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;

class PluginClassLoaderUtilsTest {

    @TempDir Path tempDir;

    @Test
    void collectJarPathsShouldIncludeConnectorsAndPlugins() throws IOException {
        Path connectorsJar = tempDir.resolve("connectors/mysql/mysql-connector.jar");
        Path pluginJar = tempDir.resolve("plugins/Jdbc/lib/postgresql-driver.jar");
        Path ignoredFile = tempDir.resolve("plugins/Jdbc/lib/README.txt");

        Files.createDirectories(connectorsJar.getParent());
        Files.createDirectories(pluginJar.getParent());
        Files.write(connectorsJar, new byte[0]);
        Files.write(pluginJar, new byte[0]);
        Files.write(ignoredFile, new byte[0]);

        List<String> pluginJars = PluginClassLoaderUtils.collectJarPaths(tempDir);

        assertEquals(
                List.of(
                        connectorsJar.toAbsolutePath().toString(),
                        pluginJar.toAbsolutePath().toString()),
                pluginJars);
    }
}
