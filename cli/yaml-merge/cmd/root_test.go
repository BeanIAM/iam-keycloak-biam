package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestYamlMergeCommand(t *testing.T) {
	// GIVEN
	// Create a temporary test directory
	const version = "v20"
	tmpDir := filepath.Join(ReleasesDir, version, LatestDir)
	err := os.MkdirAll(tmpDir, os.ModePerm)
	assert.NoError(t, err)
	//defer os.RemoveAll("releases")

	// Create test files
	downstreamFile := filepath.Join(tmpDir, PatchesDir, "ci.yml")
	upstreamFile := filepath.Join(tmpDir, KeycloakDir, "ci.yml")
	devFile := filepath.Join(tmpDir, "dev/ci.yml")
	err = os.MkdirAll(filepath.Dir(downstreamFile), os.ModePerm)
	assert.NoError(t, err)
	err = os.MkdirAll(filepath.Dir(upstreamFile), os.ModePerm)
	assert.NoError(t, err)
	//err = os.MkdirAll(filepath.Dir(devFile), os.ModePerm)
	//assert.NoError(t, err)
	downstreamData := []byte(`
on:
  workflow_call:
    inputs:
      config-path:
        required: true
        type: string
    secrets:
      envPAT:
        required: true

`)
	os.WriteFile(downstreamFile, downstreamData, os.ModePerm)

	upstreamData := []byte(`
name: Keycloak CI

on:
  push:
    branches-ignore: [main]
  # as the ci.yml contains actions that are required for PRs to be merged, it will always need to run on all PRs
  pull_request: {}
  schedule:
    - cron: '0 0 * * *'
  workflow_dispatch:

env:
  DEFAULT_JDK_VERSION: 11

concurrency:
  # Only run once for latest commit per ref and cancel other (previous) runs.
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  build:
    name: Build
    if: ${{ ( github.event_name != 'schedule' ) || ( github.event_name == 'schedule' && github.repository == 'keycloak/keycloak' ) }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-java@v3
        with:
          distribution: 'temurin'
          java-version: ${{ env.DEFAULT_JDK_VERSION }}
          cache: 'maven'
      - name: Update maven settings
        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/

      - name: Build Keycloak
        run: |
           ./mvnw clean install -nsu -B -e -DskipTests -Pdistribution
           ./mvnw clean install -nsu -B -e -f testsuite/integration-arquillian/servers/auth-server -Pauth-server-quarkus
           ./mvnw clean install -nsu -B -e -f testsuite/integration-arquillian/servers/auth-server -Pauth-server-undertow

      - name: Store Keycloak artifacts
        id: store-keycloak
        uses: actions/upload-artifact@v3
        with:
          name: keycloak-artifacts.zip
          retention-days: 1
          path: |
            ~/.m2/repository/org/keycloak
            !~/.m2/repository/org/keycloak/**/*.tar.gz

      - name: Remove keycloak artifacts before caching
        if: steps.cache.outputs.cache-hit != 'true'
        run: rm -rf ~/.m2/repository/org/keycloak

# Tests: Regular distribution

  unit-tests:
    name: Unit Tests
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-java@v3
        with:
          distribution: 'temurin'
          java-version: ${{ env.DEFAULT_JDK_VERSION }}
          cache: 'maven'
      - name: Update maven settings
        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
      - name: Cleanup org.keycloak artifacts
        run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true
      - name: Download built keycloak
        id: download-keycloak
        uses: actions/download-artifact@v3
        with:
          path: ~/.m2/repository/org/keycloak/
          name: keycloak-artifacts.zip
      - name: Run unit tests
        run: |
          if ! ./mvnw install -nsu -B -DskipTestsuite -DskipQuarkus -DskipExamples -f pom.xml; then
            find . -path '*/target/surefire-reports/*.xml' | zip -q reports-unit-tests.zip -@
            exit 1
          fi

      - name: Analyze Test and/or Coverage Results
        uses: runforesight/foresight-test-kit-action@v1.2.1
        if: always() && github.repository == 'keycloak/keycloak'
        with:
          api_key: ${{ secrets.FORESIGHT_API_KEY }}
          test_format: JUNIT
          test_framework: JUNIT
          test_path: '**/target/surefire-reports/*.xml'

      - name: Unit test reports
        uses: actions/upload-artifact@v3
        if: failure()
        with:
          name: reports-unit-tests
          retention-days: 14
          path: reports-unit-tests.zip
          if-no-files-found: ignore

  crypto-tests:
    name: Crypto Tests
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-java@v3
        with:
          distribution: 'temurin'
          java-version: ${{ env.DEFAULT_JDK_VERSION }}
          cache: 'maven'
      - name: Update maven settings
        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
      - name: Cleanup org.keycloak artifacts
        run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true
      - name: Download built keycloak
        id: download-keycloak
        uses: actions/download-artifact@v3
        with:
          path: ~/.m2/repository/org/keycloak/
          name: keycloak-artifacts.zip
      - name: Run crypto tests (BCFIPS non-approved mode)
        run: |
          if ! ./mvnw install -nsu -B -f crypto/pom.xml -Dcom.redhat.fips=true; then
            find . -path 'crypto/target/surefire-reports/*.xml' | zip -q reports-crypto-tests.zip -@
            exit 1
          fi

      - name: Run crypto tests (BCFIPS approved mode)
        run: |
          if ! ./mvnw install -nsu -B -f crypto/pom.xml -Dcom.redhat.fips=true -Dorg.bouncycastle.fips.approved_only=true; then
            find . -path 'crypto/target/surefire-reports/*.xml' | zip -q reports-crypto-tests.zip -@
            exit 1
          fi          

      - name: Crypto test reports
        uses: actions/upload-artifact@v3
        if: failure()
        with:
          name: reports-crypto-tests
          retention-days: 14
          path: reports-crypto-tests.zip
          if-no-files-found: ignore

  model-tests:
    name: Model Tests
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-java@v3
        with:
          distribution: 'temurin'
          java-version: ${{ env.DEFAULT_JDK_VERSION }}
          cache: 'maven'
      - name: Update maven settings
        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
      - name: Cleanup org.keycloak artifacts
        run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true
      - name: Download built keycloak
        id: download-keycloak
        uses: actions/download-artifact@v3
        with:
          path: ~/.m2/repository/org/keycloak/
          name: keycloak-artifacts.zip
      - name: Run model tests
        run: |
          if ! testsuite/model/test-all-profiles.sh; then
            find . -path '*/target/surefire-reports*/*.xml' | zip -q reports-model-tests.zip -@
            exit 1
          fi

      - name: Analyze Test and/or Coverage Results
        uses: runforesight/foresight-test-kit-action@v1.2.1
        if: always() && github.repository == 'keycloak/keycloak'
        with:
          api_key: ${{ secrets.FORESIGHT_API_KEY }}
          test_format: JUNIT
          test_framework: JUNIT
          test_path: 'testsuite/model/target/surefire-reports/*.xml'

      - name: Model test reports
        uses: actions/upload-artifact@v3
        if: failure()
        with:
          name: reports-model-tests
          retention-days: 14
          path: reports-model-tests.zip
          if-no-files-found: ignore

  test:
    name: Base testsuite
    needs: build
    runs-on: ubuntu-latest
    strategy:
      matrix:
        server: ['quarkus', 'quarkus-map', 'quarkus-map-hot-rod']
        tests: ['group1','group2','group3']
      fail-fast: false
    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 2

      - name: Check whether HEAD^ contains HotRod storage relevant changes
        run: echo "GIT_HOTROD_RELEVANT_DIFF=$( git diff --name-only HEAD^ | egrep -ic -e '^model/map-hot-rod|^model/map/|^model/build-processor' )" >> $GITHUB_ENV

      - name: Cache Maven packages
        if: ${{ github.event_name != 'pull_request' || matrix.server != 'quarkus-map-hot-rod' || env.GIT_HOTROD_RELEVANT_DIFF != 0 }}
        uses: actions/cache@v3
        with:
          path: ~/.m2/repository
          key: cache-2-${{ runner.os }}-m2-${{ hashFiles('**/pom.xml') }}
          restore-keys: cache-1-${{ runner.os }}-m2

      - name: Download built keycloak
        if: ${{ github.event_name != 'pull_request' || matrix.server != 'quarkus-map-hot-rod' || env.GIT_HOTROD_RELEVANT_DIFF != 0 }}
        id: download-keycloak
        uses: actions/download-artifact@v3
        with:
          path: ~/.m2/repository/org/keycloak/
          name: keycloak-artifacts.zip

      # - name: List M2 repo
      #   run: |
      #     find ~ -name *dist*.zip
      #     ls -lR ~/.m2/repository

      - uses: actions/setup-java@v3
        if: ${{ github.event_name != 'pull_request' || matrix.server != 'quarkus-map-hot-rod' || env.GIT_HOTROD_RELEVANT_DIFF != 0 }}
        with:
          distribution: 'temurin'
          java-version: ${{ env.DEFAULT_JDK_VERSION }}
      - name: Update maven settings
        if: ${{ github.event_name != 'pull_request' || matrix.server != 'quarkus-map-hot-rod' || env.GIT_HOTROD_RELEVANT_DIFF != 0 }}
        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
      - name: Prepare test providers
        if: ${{ matrix.server == 'quarkus' || matrix.server == 'quarkus-map' }}
        run: ./mvnw clean install -nsu -B -e -f testsuite/integration-arquillian/servers/auth-server/services/testsuite-providers -Pauth-server-quarkus
      - name: Run base tests
        if: ${{ github.event_name != 'pull_request' || matrix.server != 'quarkus-map-hot-rod' || env.GIT_HOTROD_RELEVANT_DIFF != 0 }}
        run: |
          declare -A PARAMS TESTGROUP
          PARAMS["quarkus"]="-Pauth-server-quarkus"
          PARAMS["quarkus-map"]="-Pauth-server-quarkus -Pmap-storage -Dpageload.timeout=90000"
          PARAMS["quarkus-map-hot-rod"]="-Pauth-server-quarkus -Pmap-storage,map-storage-hot-rod -Dpageload.timeout=90000"
          TESTGROUP["group1"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.(a[abc]|ad[a-l]|[^a-q]).*]"   # Tests alphabetically before admin tests and those after "r"
          TESTGROUP["group2"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.(ad[^a-l]|a[^a-d]|b).*]"      # Admin tests and those starting with "b"
          TESTGROUP["group3"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.([c-q]).*]"                   # All the rest

          ./mvnw clean install -nsu -B ${PARAMS["${{ matrix.server }}"]} ${TESTGROUP["${{ matrix.tests }}"]} -f testsuite/integration-arquillian/tests/base/pom.xml | misc/log/trimmer.sh

          TEST_RESULT=${PIPESTATUS[0]}
          find . -path '*/target/surefire-reports/*.xml' | zip -q reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip -@
          exit $TEST_RESULT

      - name: Analyze Test and/or Coverage Results
        uses: runforesight/foresight-test-kit-action@v1.2.1
        if: always() && github.repository == 'keycloak/keycloak'
        with:
          api_key: ${{ secrets.FORESIGHT_API_KEY }}
          test_format: JUNIT
          test_framework: JUNIT
          test_path: 'testsuite/integration-arquillian/tests/base/target/surefire-reports/*.xml'

      - name: Base test reports
        uses: actions/upload-artifact@v3
        if: failure()
        with:
          name: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}
          retention-days: 14
          path: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip
          if-no-files-found: ignore

  test-fips:
    name: Base testsuite (fips)
    needs: build
    runs-on: ubuntu-latest
    strategy:
      matrix:
        server: ['bcfips-nonapproved-pkcs12']
        tests: ['group1']
      fail-fast: false
    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 2

      - name: Cache Maven packages
        uses: actions/cache@v3
        with:
          path: ~/.m2/repository
          key: cache-2-${{ runner.os }}-m2-${{ hashFiles('**/pom.xml') }}
          restore-keys: cache-1-${{ runner.os }}-m2

      - name: Download built keycloak
        id: download-keycloak
        uses: actions/download-artifact@v3
        with:
          path: ~/.m2/repository/org/keycloak/
          name: keycloak-artifacts.zip

      # - name: List M2 repo
      #   run: |
      #     find ~ -name *dist*.zip
      #     ls -lR ~/.m2/repository

      - uses: actions/setup-java@v3
        with:
          distribution: 'temurin'
          java-version: ${{ env.DEFAULT_JDK_VERSION }}
      - name: Update maven settings
        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
      - name: Prepare quarkus distribution with BCFIPS
        run: ./mvnw clean install -nsu -B -e -f testsuite/integration-arquillian/servers/auth-server/quarkus -Pauth-server-quarkus,auth-server-fips140-2
      - name: Run base tests
        run: |
          declare -A PARAMS TESTGROUP
          PARAMS["bcfips-nonapproved-pkcs12"]="-Pauth-server-quarkus,auth-server-fips140-2"
          # Tests in the package "forms" and some keystore related tests
          TESTGROUP["group1"]="-Dtest=org.keycloak.testsuite.forms.**,ClientAuthSignedJWTTest,CredentialsTest,JavaKeystoreKeyProviderTest,ServerInfoTest"
          
          ./mvnw clean install -nsu -B ${PARAMS["${{ matrix.server }}"]} ${TESTGROUP["${{ matrix.tests }}"]} -f testsuite/integration-arquillian/tests/base/pom.xml | misc/log/trimmer.sh
          
          TEST_RESULT=${PIPESTATUS[0]}
          find . -path '*/target/surefire-reports/*.xml' | zip -q reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip -@
          exit $TEST_RESULT

      - name: Analyze Test and/or Coverage Results
        uses: runforesight/foresight-test-kit-action@v1.2.1
        if: always() && github.repository == 'keycloak/keycloak'
        with:
          api_key: ${{ secrets.FORESIGHT_API_KEY }}
          test_format: JUNIT
          test_framework: JUNIT
          test_path: 'testsuite/integration-arquillian/tests/base/target/surefire-reports/*.xml'

      - name: Base test reports
        uses: actions/upload-artifact@v3
        if: failure()
        with:
          name: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}
          retention-days: 14
          path: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip
          if-no-files-found: ignore

  test-posgres:
    name: Base testsuite (postgres)
    needs: build
    runs-on: ubuntu-latest
    strategy:
      matrix:
        server: ['undertow-map-jpa']
        tests: ['group1','group2','group3']
      fail-fast: false

    services:
      # Label used to access the service container
      postgres:
        # Docker Hub image
        image: postgres
        env:
          # Provide env variables for the image
          POSTGRES_DB: keycloak
          POSTGRES_USER: keycloak
          POSTGRES_PASSWORD: pass
        # Set health checks to wait until postgres has started
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          # Maps tcp port 5432 on service container to the host
          - 5432:5432

    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 2

      - name: Check whether HEAD^ contains JPA map storage relevant changes
        run: echo "GIT_MAP_JPA_RELEVANT_DIFF=$( git diff --name-only HEAD^ | egrep -ic -e '^model/map-jpa/|^model/map/|^model/build-processor' )" >> $GITHUB_ENV

      - name: Cache Maven packages
        if: ${{ github.event_name != 'pull_request' || env.GIT_MAP_JPA_RELEVANT_DIFF != 0 }}
        uses: actions/cache@v3
        with:
          path: ~/.m2/repository
          key: cache-2-${{ runner.os }}-m2-${{ hashFiles('**/pom.xml') }}
          restore-keys: cache-1-${{ runner.os }}-m2

      - name: Download built keycloak
        if: ${{ github.event_name != 'pull_request' || env.GIT_MAP_JPA_RELEVANT_DIFF != 0 }}
        id: download-keycloak
        uses: actions/download-artifact@v3
        with:
          path: ~/.m2/repository/org/keycloak/
          name: keycloak-artifacts.zip

      - uses: actions/setup-java@v3
        if: ${{ github.event_name != 'pull_request' || env.GIT_MAP_JPA_RELEVANT_DIFF != 0 }}
        with:
          distribution: 'temurin'
          java-version: ${{ env.DEFAULT_JDK_VERSION }}
      - name: Update maven settings
        if: ${{ github.event_name != 'pull_request' || env.GIT_MAP_JPA_RELEVANT_DIFF != 0 }}
        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/

      - name: Run base tests
        if: ${{ github.event_name != 'pull_request' || env.GIT_MAP_JPA_RELEVANT_DIFF != 0 }}
        run: |
          declare -A PARAMS TESTGROUP
          PARAMS["undertow-map-jpa"]="-Pmap-storage,map-storage-jpa -Dpageload.timeout=90000"
          TESTGROUP["group1"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.(a[abc]|ad[a-l]|[^a-q]).*]"   # Tests alphabetically before admin tests and those after "r"
          TESTGROUP["group2"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.(ad[^a-l]|a[^a-d]|b).*]"      # Admin tests and those starting with "b"
          TESTGROUP["group3"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.([c-q]).*]"                   # All the rest

          ./mvnw clean install -nsu -B ${PARAMS["${{ matrix.server }}"]} ${TESTGROUP["${{ matrix.tests }}"]} -f testsuite/integration-arquillian/tests/base/pom.xml | misc/log/trimmer.sh

          TEST_RESULT=${PIPESTATUS[0]}
          find . -path '*/target/surefire-reports/*.xml' | zip -q reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip -@
          exit $TEST_RESULT

      - name: Analyze Test and/or Coverage Results
        uses: runforesight/foresight-test-kit-action@v1.2.1
        if: always() && github.repository == 'keycloak/keycloak'
        with:
          api_key: ${{ secrets.FORESIGHT_API_KEY }}
          test_format: JUNIT
          test_framework: JUNIT
          test_path: 'testsuite/integration-arquillian/tests/base/target/surefire-reports/*.xml'

      - name: Base test reports
        uses: actions/upload-artifact@v3
        if: failure()
        with:
          name: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}
          retention-days: 14
          path: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip
          if-no-files-found: ignore

### Tests: Quarkus distribution

  quarkus-test-cluster:
    name: Quarkus Test Clustering
    needs: build
    runs-on: ubuntu-latest
    env:
      MAVEN_OPTS: -Xmx1024m
    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-java@v3
        with:
          distribution: 'temurin'
          java-version: ${{ env.DEFAULT_JDK_VERSION }}
          cache: 'maven'

      - name: Cleanup org.keycloak artifacts
        run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true

      - name: Download built keycloak
        id: download-keycloak
        uses: actions/download-artifact@v3
        with:
          path: ~/.m2/repository/org/keycloak/
          name: keycloak-artifacts.zip
      - uses: actions/setup-java@v3
        with:
          distribution: 'temurin'
          java-version: ${{ env.DEFAULT_JDK_VERSION }}
      - name: Update maven settings
        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
      - name: Run Quarkus cluster tests
        run: |
          echo '::group::Compiling testsuite'
          ./mvnw clean install -nsu -B -Pauth-server-quarkus -DskipTests -f testsuite/pom.xml
          echo '::endgroup::'
          ./mvnw clean install -nsu -B -Pauth-server-cluster-quarkus -Dsession.cache.owners=2 -Dtest=**.cluster.** -f testsuite/integration-arquillian/pom.xml  | misc/log/trimmer.sh
          TEST_RESULT=${PIPESTATUS[0]}
          find . -path '*/target/surefire-reports/*.xml' | zip -q reports-quarkus-cluster-tests.zip -@
          exit $TEST_RESULT

      - name: Analyze Test and/or Coverage Results
        uses: runforesight/foresight-test-kit-action@v1.2.1
        if: always() && github.repository == 'keycloak/keycloak'
        with:
          api_key: ${{ secrets.FORESIGHT_API_KEY }}
          test_format: JUNIT
          test_framework: JUNIT
          test_path: 'testsuite/integration-arquillian/tests/base/target/surefire-reports/*.xml'

      - name: Quarkus cluster test reports
        uses: actions/upload-artifact@v3
        if: failure()
        with:
          name: reports-quarkus-cluster-tests
          retention-days: 14
          path: reports-quarkus-cluster-tests.zip
          if-no-files-found: ignore

  ### Tests: Quarkus distribution

  quarkus-tests:
    name: Quarkus Tests
    needs: build
    runs-on: ubuntu-latest
    env:
      MAVEN_OPTS: -Xmx1024m
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-java@v3
        with:
          distribution: 'temurin'
          java-version: ${{ env.DEFAULT_JDK_VERSION }}
          cache: 'maven'
      - name: Cleanup org.keycloak artifacts
        run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true

      - name: Download built keycloak
        id: download-keycloak
        uses: actions/download-artifact@v3
        with:
          path: ~/.m2/repository/org/keycloak/
          name: keycloak-artifacts.zip
      - name: Update maven settings
        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/

      - name: Prepare the local distribution archives
        run: ./mvnw clean install -DskipTests -Pdistribution

      - name: Run Quarkus Integration Tests
        run: |
          ./mvnw clean install -nsu -B -f quarkus/tests/pom.xml | misc/log/trimmer.sh
          TEST_RESULT=${PIPESTATUS[0]}
          find . -path '*/target/surefire-reports/*.xml' | zip -q reports-quarkus-tests.zip -@
          exit $TEST_RESULT

      - name: Run Quarkus Storage Tests
        run: |
          ./mvnw clean install -nsu -B -f quarkus/tests/pom.xml -Ptest-database -Dtest=PostgreSQLDistTest,MariaDBDistTest#testSuccessful,MySQLDistTest#testSuccessful,DatabaseOptionsDistTest,JPAStoreDistTest,HotRodStoreDistTest,MixedStoreDistTest | misc/log/trimmer.sh
          TEST_RESULT=${PIPESTATUS[0]}
          find . -path '*/target/surefire-reports/*.xml' | zip -q reports-quarkus-tests.zip -@
          exit $TEST_RESULT

      - name: Run Quarkus Tests in Docker
        run: |
          ./mvnw clean install -nsu -B -f quarkus/tests/pom.xml -Dkc.quarkus.tests.dist=docker -Dtest=StartCommandDistTest | misc/log/trimmer.sh
          TEST_RESULT=${PIPESTATUS[0]}
          exit $TEST_RESULT

      - name: Analyze Test and/or Coverage Results
        uses: runforesight/foresight-test-kit-action@v1.2.1
        if: always() && github.repository == 'keycloak/keycloak'
        with:
          api_key: ${{ secrets.FORESIGHT_API_KEY }}
          test_format: JUNIT
          test_framework: JUNIT
          test_path: 'quarkus/tests/integration/target/surefire-reports/*.xml'

      - name: Quarkus test reports
        uses: actions/upload-artifact@v3
        if: failure()
        with:
          name: reports-quarkus-tests
          retention-days: 14
          path: reports-quarkus-tests.zip
          if-no-files-found: ignore

# NOTE: WebAuthn tests can be enabled once the issue #12621 is resolved
#
#  webauthn-test:
#    name: WebAuthn Tests
#    needs: build
#    runs-on: ubuntu-latest
#    steps:
#      - uses: actions/checkout@v2
#        with:
#          fetch-depth: 2
#
#      - name: Check whether this phase should run
#        run: echo "GIT_DIFF=$[ $( git diff --name-only HEAD^ | egrep -ic 'webauthn|passwordless' ) ]" >> $GITHUB_ENV
#
#      - uses: actions/setup-java@v1
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        with:
#          java-version: ${{ env.DEFAULT_JDK_VERSION }}
#
#      - name: Update maven settings
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
#
#      - name: Cache Maven packages
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        uses: actions/cache@v2
#        with:
#          path: ~/.m2/repository
#          key: cache-1-${{ runner.os }}-m2-${{ hashFiles('**/pom.xml') }}
#          restore-keys: cache-1-${{ runner.os }}-m2
#
#      - name: Cleanup org.keycloak artifacts
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true
#
#      - name: Download built keycloak
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        id: download-keycloak
#        uses: actions/download-artifact@v2
#        with:
#          path: ~/.m2/repository/org/keycloak/
#          name: keycloak-artifacts.zip
#
#      - name: Run WebAuthn tests
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        run: |
#          mvn clean install -nsu -B -Dbrowser=chrome -Pwebauthn -f testsuite/integration-arquillian/tests/other/pom.xml -Dtest=org.keycloak.testsuite.webauthn.**.*Test | misc/log/trimmer.sh
#
#          TEST_RESULT=${PIPESTATUS[0]}
#          find . -path '*/target/surefire-reports/*.xml' | zip -q reports-webauthn-tests.zip -@
#          exit $TEST_RESULT
#
#      - name: WebAuthn test reports
#        uses: actions/upload-artifact@v2
#        if: failure()
#        with:
#          name: reports-webauthn-tests
#          retention-days: 14
#          path: reports-webauthn-tests.zip
#          if-no-files-found: ignore
`)
	os.WriteFile(upstreamFile, upstreamData, os.ModePerm)

	// Run the command
	args := []string{version}
	rootCmd.SetArgs(args)
	err = rootCmd.RunE(nil, args)
	assert.NoError(t, err)

	// Assert that the dev file was written correctly
	devData, err := os.ReadFile(devFile)
	assert.NoError(t, err)

	expectedData := `name: Keycloak CI
on:
    push:
        branches-ignore: [main]
    # as the ci.yml contains actions that are required for PRs to be merged, it will always need to run on all PRs
    pull_request: {}
    schedule:
        - cron: '0 0 * * *'
    workflow_dispatch:
    workflow_call:
        inputs:
            config-path:
                required: true
                type: string
        secrets:
            envPAT:
                required: true
env:
    DEFAULT_JDK_VERSION: 11
concurrency:
    # Only run once for latest commit per ref and cancel other (previous) runs.
    group: ${{ github.workflow }}-${{ github.ref }}
    cancel-in-progress: true
jobs:
    build:
        name: Build
        if: ${{ ( github.event_name != 'schedule' ) || ( github.event_name == 'schedule' && github.repository == 'keycloak/keycloak' ) }}
        runs-on: ubuntu-latest
        steps:
            - uses: actions/checkout@v3
            - uses: actions/setup-java@v3
              with:
                distribution: 'temurin'
                java-version: ${{ env.DEFAULT_JDK_VERSION }}
                cache: 'maven'
            - name: Update maven settings
              run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
            - name: Build Keycloak
              run: |
                ./mvnw clean install -nsu -B -e -DskipTests -Pdistribution
                ./mvnw clean install -nsu -B -e -f testsuite/integration-arquillian/servers/auth-server -Pauth-server-quarkus
                ./mvnw clean install -nsu -B -e -f testsuite/integration-arquillian/servers/auth-server -Pauth-server-undertow
            - name: Store Keycloak artifacts
              id: store-keycloak
              uses: actions/upload-artifact@v3
              with:
                name: keycloak-artifacts.zip
                retention-days: 1
                path: |
                    ~/.m2/repository/org/keycloak
                    !~/.m2/repository/org/keycloak/**/*.tar.gz
            - name: Remove keycloak artifacts before caching
              if: steps.cache.outputs.cache-hit != 'true'
              run: rm -rf ~/.m2/repository/org/keycloak
              # Tests: Regular distribution
    unit-tests:
        name: Unit Tests
        runs-on: ubuntu-latest
        needs: build
        steps:
            - uses: actions/checkout@v3
            - uses: actions/setup-java@v3
              with:
                distribution: 'temurin'
                java-version: ${{ env.DEFAULT_JDK_VERSION }}
                cache: 'maven'
            - name: Update maven settings
              run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
            - name: Cleanup org.keycloak artifacts
              run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true
            - name: Download built keycloak
              id: download-keycloak
              uses: actions/download-artifact@v3
              with:
                path: ~/.m2/repository/org/keycloak/
                name: keycloak-artifacts.zip
            - name: Run unit tests
              run: |
                if ! ./mvnw install -nsu -B -DskipTestsuite -DskipQuarkus -DskipExamples -f pom.xml; then
                  find . -path '*/target/surefire-reports/*.xml' | zip -q reports-unit-tests.zip -@
                  exit 1
                fi
            - name: Analyze Test and/or Coverage Results
              uses: runforesight/foresight-test-kit-action@v1.2.1
              if: always() && github.repository == 'keycloak/keycloak'
              with:
                api_key: ${{ secrets.FORESIGHT_API_KEY }}
                test_format: JUNIT
                test_framework: JUNIT
                test_path: '**/target/surefire-reports/*.xml'
            - name: Unit test reports
              uses: actions/upload-artifact@v3
              if: failure()
              with:
                name: reports-unit-tests
                retention-days: 14
                path: reports-unit-tests.zip
                if-no-files-found: ignore
    crypto-tests:
        name: Crypto Tests
        runs-on: ubuntu-latest
        needs: build
        steps:
            - uses: actions/checkout@v3
            - uses: actions/setup-java@v3
              with:
                distribution: 'temurin'
                java-version: ${{ env.DEFAULT_JDK_VERSION }}
                cache: 'maven'
            - name: Update maven settings
              run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
            - name: Cleanup org.keycloak artifacts
              run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true
            - name: Download built keycloak
              id: download-keycloak
              uses: actions/download-artifact@v3
              with:
                path: ~/.m2/repository/org/keycloak/
                name: keycloak-artifacts.zip
            - name: Run crypto tests (BCFIPS non-approved mode)
              run: |
                if ! ./mvnw install -nsu -B -f crypto/pom.xml -Dcom.redhat.fips=true; then
                  find . -path 'crypto/target/surefire-reports/*.xml' | zip -q reports-crypto-tests.zip -@
                  exit 1
                fi
            - name: Run crypto tests (BCFIPS approved mode)
              run: "if ! ./mvnw install -nsu -B -f crypto/pom.xml -Dcom.redhat.fips=true -Dorg.bouncycastle.fips.approved_only=true; then\n  find . -path 'crypto/target/surefire-reports/*.xml' | zip -q reports-crypto-tests.zip -@\n  exit 1\nfi          \n"
            - name: Crypto test reports
              uses: actions/upload-artifact@v3
              if: failure()
              with:
                name: reports-crypto-tests
                retention-days: 14
                path: reports-crypto-tests.zip
                if-no-files-found: ignore
    model-tests:
        name: Model Tests
        runs-on: ubuntu-latest
        needs: build
        steps:
            - uses: actions/checkout@v3
            - uses: actions/setup-java@v3
              with:
                distribution: 'temurin'
                java-version: ${{ env.DEFAULT_JDK_VERSION }}
                cache: 'maven'
            - name: Update maven settings
              run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
            - name: Cleanup org.keycloak artifacts
              run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true
            - name: Download built keycloak
              id: download-keycloak
              uses: actions/download-artifact@v3
              with:
                path: ~/.m2/repository/org/keycloak/
                name: keycloak-artifacts.zip
            - name: Run model tests
              run: |
                if ! testsuite/model/test-all-profiles.sh; then
                  find . -path '*/target/surefire-reports*/*.xml' | zip -q reports-model-tests.zip -@
                  exit 1
                fi
            - name: Analyze Test and/or Coverage Results
              uses: runforesight/foresight-test-kit-action@v1.2.1
              if: always() && github.repository == 'keycloak/keycloak'
              with:
                api_key: ${{ secrets.FORESIGHT_API_KEY }}
                test_format: JUNIT
                test_framework: JUNIT
                test_path: 'testsuite/model/target/surefire-reports/*.xml'
            - name: Model test reports
              uses: actions/upload-artifact@v3
              if: failure()
              with:
                name: reports-model-tests
                retention-days: 14
                path: reports-model-tests.zip
                if-no-files-found: ignore
    test:
        name: Base testsuite
        needs: build
        runs-on: ubuntu-latest
        strategy:
            matrix:
                server: ['quarkus', 'quarkus-map', 'quarkus-map-hot-rod']
                tests: ['group1', 'group2', 'group3']
            fail-fast: false
        steps:
            - uses: actions/checkout@v3
              with:
                fetch-depth: 2
            - name: Check whether HEAD^ contains HotRod storage relevant changes
              run: echo "GIT_HOTROD_RELEVANT_DIFF=$( git diff --name-only HEAD^ | egrep -ic -e '^model/map-hot-rod|^model/map/|^model/build-processor' )" >> $GITHUB_ENV
            - name: Cache Maven packages
              if: ${{ github.event_name != 'pull_request' || matrix.server != 'quarkus-map-hot-rod' || env.GIT_HOTROD_RELEVANT_DIFF != 0 }}
              uses: actions/cache@v3
              with:
                path: ~/.m2/repository
                key: cache-2-${{ runner.os }}-m2-${{ hashFiles('**/pom.xml') }}
                restore-keys: cache-1-${{ runner.os }}-m2
            - name: Download built keycloak
              if: ${{ github.event_name != 'pull_request' || matrix.server != 'quarkus-map-hot-rod' || env.GIT_HOTROD_RELEVANT_DIFF != 0 }}
              id: download-keycloak
              uses: actions/download-artifact@v3
              with:
                path: ~/.m2/repository/org/keycloak/
                name: keycloak-artifacts.zip
            # - name: List M2 repo
            #   run: |
            #     find ~ -name *dist*.zip
            #     ls -lR ~/.m2/repository
            - uses: actions/setup-java@v3
              if: ${{ github.event_name != 'pull_request' || matrix.server != 'quarkus-map-hot-rod' || env.GIT_HOTROD_RELEVANT_DIFF != 0 }}
              with:
                distribution: 'temurin'
                java-version: ${{ env.DEFAULT_JDK_VERSION }}
            - name: Update maven settings
              if: ${{ github.event_name != 'pull_request' || matrix.server != 'quarkus-map-hot-rod' || env.GIT_HOTROD_RELEVANT_DIFF != 0 }}
              run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
            - name: Prepare test providers
              if: ${{ matrix.server == 'quarkus' || matrix.server == 'quarkus-map' }}
              run: ./mvnw clean install -nsu -B -e -f testsuite/integration-arquillian/servers/auth-server/services/testsuite-providers -Pauth-server-quarkus
            - name: Run base tests
              if: ${{ github.event_name != 'pull_request' || matrix.server != 'quarkus-map-hot-rod' || env.GIT_HOTROD_RELEVANT_DIFF != 0 }}
              run: |
                declare -A PARAMS TESTGROUP
                PARAMS["quarkus"]="-Pauth-server-quarkus"
                PARAMS["quarkus-map"]="-Pauth-server-quarkus -Pmap-storage -Dpageload.timeout=90000"
                PARAMS["quarkus-map-hot-rod"]="-Pauth-server-quarkus -Pmap-storage,map-storage-hot-rod -Dpageload.timeout=90000"
                TESTGROUP["group1"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.(a[abc]|ad[a-l]|[^a-q]).*]"   # Tests alphabetically before admin tests and those after "r"
                TESTGROUP["group2"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.(ad[^a-l]|a[^a-d]|b).*]"      # Admin tests and those starting with "b"
                TESTGROUP["group3"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.([c-q]).*]"                   # All the rest

                ./mvnw clean install -nsu -B ${PARAMS["${{ matrix.server }}"]} ${TESTGROUP["${{ matrix.tests }}"]} -f testsuite/integration-arquillian/tests/base/pom.xml | misc/log/trimmer.sh

                TEST_RESULT=${PIPESTATUS[0]}
                find . -path '*/target/surefire-reports/*.xml' | zip -q reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip -@
                exit $TEST_RESULT
            - name: Analyze Test and/or Coverage Results
              uses: runforesight/foresight-test-kit-action@v1.2.1
              if: always() && github.repository == 'keycloak/keycloak'
              with:
                api_key: ${{ secrets.FORESIGHT_API_KEY }}
                test_format: JUNIT
                test_framework: JUNIT
                test_path: 'testsuite/integration-arquillian/tests/base/target/surefire-reports/*.xml'
            - name: Base test reports
              uses: actions/upload-artifact@v3
              if: failure()
              with:
                name: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}
                retention-days: 14
                path: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip
                if-no-files-found: ignore
    test-fips:
        name: Base testsuite (fips)
        needs: build
        runs-on: ubuntu-latest
        strategy:
            matrix:
                server: ['bcfips-nonapproved-pkcs12']
                tests: ['group1']
            fail-fast: false
        steps:
            - uses: actions/checkout@v3
              with:
                fetch-depth: 2
            - name: Cache Maven packages
              uses: actions/cache@v3
              with:
                path: ~/.m2/repository
                key: cache-2-${{ runner.os }}-m2-${{ hashFiles('**/pom.xml') }}
                restore-keys: cache-1-${{ runner.os }}-m2
            - name: Download built keycloak
              id: download-keycloak
              uses: actions/download-artifact@v3
              with:
                path: ~/.m2/repository/org/keycloak/
                name: keycloak-artifacts.zip
            # - name: List M2 repo
            #   run: |
            #     find ~ -name *dist*.zip
            #     ls -lR ~/.m2/repository
            - uses: actions/setup-java@v3
              with:
                distribution: 'temurin'
                java-version: ${{ env.DEFAULT_JDK_VERSION }}
            - name: Update maven settings
              run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
            - name: Prepare quarkus distribution with BCFIPS
              run: ./mvnw clean install -nsu -B -e -f testsuite/integration-arquillian/servers/auth-server/quarkus -Pauth-server-quarkus,auth-server-fips140-2
            - name: Run base tests
              run: |
                declare -A PARAMS TESTGROUP
                PARAMS["bcfips-nonapproved-pkcs12"]="-Pauth-server-quarkus,auth-server-fips140-2"
                # Tests in the package "forms" and some keystore related tests
                TESTGROUP["group1"]="-Dtest=org.keycloak.testsuite.forms.**,ClientAuthSignedJWTTest,CredentialsTest,JavaKeystoreKeyProviderTest,ServerInfoTest"

                ./mvnw clean install -nsu -B ${PARAMS["${{ matrix.server }}"]} ${TESTGROUP["${{ matrix.tests }}"]} -f testsuite/integration-arquillian/tests/base/pom.xml | misc/log/trimmer.sh

                TEST_RESULT=${PIPESTATUS[0]}
                find . -path '*/target/surefire-reports/*.xml' | zip -q reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip -@
                exit $TEST_RESULT
            - name: Analyze Test and/or Coverage Results
              uses: runforesight/foresight-test-kit-action@v1.2.1
              if: always() && github.repository == 'keycloak/keycloak'
              with:
                api_key: ${{ secrets.FORESIGHT_API_KEY }}
                test_format: JUNIT
                test_framework: JUNIT
                test_path: 'testsuite/integration-arquillian/tests/base/target/surefire-reports/*.xml'
            - name: Base test reports
              uses: actions/upload-artifact@v3
              if: failure()
              with:
                name: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}
                retention-days: 14
                path: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip
                if-no-files-found: ignore
    test-posgres:
        name: Base testsuite (postgres)
        needs: build
        runs-on: ubuntu-latest
        strategy:
            matrix:
                server: ['undertow-map-jpa']
                tests: ['group1', 'group2', 'group3']
            fail-fast: false
        services:
            # Label used to access the service container
            postgres:
                # Docker Hub image
                image: postgres
                env:
                    # Provide env variables for the image
                    POSTGRES_DB: keycloak
                    POSTGRES_USER: keycloak
                    POSTGRES_PASSWORD: pass
                # Set health checks to wait until postgres has started
                options: >-
                    --health-cmd pg_isready --health-interval 10s --health-timeout 5s --health-retries 5
                ports:
                    # Maps tcp port 5432 on service container to the host
                    - 5432:5432
        steps:
            - uses: actions/checkout@v3
              with:
                fetch-depth: 2
            - name: Check whether HEAD^ contains JPA map storage relevant changes
              run: echo "GIT_MAP_JPA_RELEVANT_DIFF=$( git diff --name-only HEAD^ | egrep -ic -e '^model/map-jpa/|^model/map/|^model/build-processor' )" >> $GITHUB_ENV
            - name: Cache Maven packages
              if: ${{ github.event_name != 'pull_request' || env.GIT_MAP_JPA_RELEVANT_DIFF != 0 }}
              uses: actions/cache@v3
              with:
                path: ~/.m2/repository
                key: cache-2-${{ runner.os }}-m2-${{ hashFiles('**/pom.xml') }}
                restore-keys: cache-1-${{ runner.os }}-m2
            - name: Download built keycloak
              if: ${{ github.event_name != 'pull_request' || env.GIT_MAP_JPA_RELEVANT_DIFF != 0 }}
              id: download-keycloak
              uses: actions/download-artifact@v3
              with:
                path: ~/.m2/repository/org/keycloak/
                name: keycloak-artifacts.zip
            - uses: actions/setup-java@v3
              if: ${{ github.event_name != 'pull_request' || env.GIT_MAP_JPA_RELEVANT_DIFF != 0 }}
              with:
                distribution: 'temurin'
                java-version: ${{ env.DEFAULT_JDK_VERSION }}
            - name: Update maven settings
              if: ${{ github.event_name != 'pull_request' || env.GIT_MAP_JPA_RELEVANT_DIFF != 0 }}
              run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
            - name: Run base tests
              if: ${{ github.event_name != 'pull_request' || env.GIT_MAP_JPA_RELEVANT_DIFF != 0 }}
              run: |
                declare -A PARAMS TESTGROUP
                PARAMS["undertow-map-jpa"]="-Pmap-storage,map-storage-jpa -Dpageload.timeout=90000"
                TESTGROUP["group1"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.(a[abc]|ad[a-l]|[^a-q]).*]"   # Tests alphabetically before admin tests and those after "r"
                TESTGROUP["group2"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.(ad[^a-l]|a[^a-d]|b).*]"      # Admin tests and those starting with "b"
                TESTGROUP["group3"]="-Dtest=!**.crossdc.**,!**.cluster.**,%regex[org.keycloak.testsuite.([c-q]).*]"                   # All the rest

                ./mvnw clean install -nsu -B ${PARAMS["${{ matrix.server }}"]} ${TESTGROUP["${{ matrix.tests }}"]} -f testsuite/integration-arquillian/tests/base/pom.xml | misc/log/trimmer.sh

                TEST_RESULT=${PIPESTATUS[0]}
                find . -path '*/target/surefire-reports/*.xml' | zip -q reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip -@
                exit $TEST_RESULT
            - name: Analyze Test and/or Coverage Results
              uses: runforesight/foresight-test-kit-action@v1.2.1
              if: always() && github.repository == 'keycloak/keycloak'
              with:
                api_key: ${{ secrets.FORESIGHT_API_KEY }}
                test_format: JUNIT
                test_framework: JUNIT
                test_path: 'testsuite/integration-arquillian/tests/base/target/surefire-reports/*.xml'
            - name: Base test reports
              uses: actions/upload-artifact@v3
              if: failure()
              with:
                name: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}
                retention-days: 14
                path: reports-${{ matrix.server }}-base-tests-${{ matrix.tests }}.zip
                if-no-files-found: ignore
                ### Tests: Quarkus distribution
    quarkus-test-cluster:
        name: Quarkus Test Clustering
        needs: build
        runs-on: ubuntu-latest
        env:
            MAVEN_OPTS: -Xmx1024m
        steps:
            - uses: actions/checkout@v3
            - uses: actions/setup-java@v3
              with:
                distribution: 'temurin'
                java-version: ${{ env.DEFAULT_JDK_VERSION }}
                cache: 'maven'
            - name: Cleanup org.keycloak artifacts
              run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true
            - name: Download built keycloak
              id: download-keycloak
              uses: actions/download-artifact@v3
              with:
                path: ~/.m2/repository/org/keycloak/
                name: keycloak-artifacts.zip
            - uses: actions/setup-java@v3
              with:
                distribution: 'temurin'
                java-version: ${{ env.DEFAULT_JDK_VERSION }}
            - name: Update maven settings
              run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
            - name: Run Quarkus cluster tests
              run: |
                echo '::group::Compiling testsuite'
                ./mvnw clean install -nsu -B -Pauth-server-quarkus -DskipTests -f testsuite/pom.xml
                echo '::endgroup::'
                ./mvnw clean install -nsu -B -Pauth-server-cluster-quarkus -Dsession.cache.owners=2 -Dtest=**.cluster.** -f testsuite/integration-arquillian/pom.xml  | misc/log/trimmer.sh
                TEST_RESULT=${PIPESTATUS[0]}
                find . -path '*/target/surefire-reports/*.xml' | zip -q reports-quarkus-cluster-tests.zip -@
                exit $TEST_RESULT
            - name: Analyze Test and/or Coverage Results
              uses: runforesight/foresight-test-kit-action@v1.2.1
              if: always() && github.repository == 'keycloak/keycloak'
              with:
                api_key: ${{ secrets.FORESIGHT_API_KEY }}
                test_format: JUNIT
                test_framework: JUNIT
                test_path: 'testsuite/integration-arquillian/tests/base/target/surefire-reports/*.xml'
            - name: Quarkus cluster test reports
              uses: actions/upload-artifact@v3
              if: failure()
              with:
                name: reports-quarkus-cluster-tests
                retention-days: 14
                path: reports-quarkus-cluster-tests.zip
                if-no-files-found: ignore
    ### Tests: Quarkus distribution
    quarkus-tests:
        name: Quarkus Tests
        needs: build
        runs-on: ubuntu-latest
        env:
            MAVEN_OPTS: -Xmx1024m
        steps:
            - uses: actions/checkout@v3
            - uses: actions/setup-java@v3
              with:
                distribution: 'temurin'
                java-version: ${{ env.DEFAULT_JDK_VERSION }}
                cache: 'maven'
            - name: Cleanup org.keycloak artifacts
              run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true
            - name: Download built keycloak
              id: download-keycloak
              uses: actions/download-artifact@v3
              with:
                path: ~/.m2/repository/org/keycloak/
                name: keycloak-artifacts.zip
            - name: Update maven settings
              run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
            - name: Prepare the local distribution archives
              run: ./mvnw clean install -DskipTests -Pdistribution
            - name: Run Quarkus Integration Tests
              run: |
                ./mvnw clean install -nsu -B -f quarkus/tests/pom.xml | misc/log/trimmer.sh
                TEST_RESULT=${PIPESTATUS[0]}
                find . -path '*/target/surefire-reports/*.xml' | zip -q reports-quarkus-tests.zip -@
                exit $TEST_RESULT
            - name: Run Quarkus Storage Tests
              run: |
                ./mvnw clean install -nsu -B -f quarkus/tests/pom.xml -Ptest-database -Dtest=PostgreSQLDistTest,MariaDBDistTest#testSuccessful,MySQLDistTest#testSuccessful,DatabaseOptionsDistTest,JPAStoreDistTest,HotRodStoreDistTest,MixedStoreDistTest | misc/log/trimmer.sh
                TEST_RESULT=${PIPESTATUS[0]}
                find . -path '*/target/surefire-reports/*.xml' | zip -q reports-quarkus-tests.zip -@
                exit $TEST_RESULT
            - name: Run Quarkus Tests in Docker
              run: |
                ./mvnw clean install -nsu -B -f quarkus/tests/pom.xml -Dkc.quarkus.tests.dist=docker -Dtest=StartCommandDistTest | misc/log/trimmer.sh
                TEST_RESULT=${PIPESTATUS[0]}
                exit $TEST_RESULT
            - name: Analyze Test and/or Coverage Results
              uses: runforesight/foresight-test-kit-action@v1.2.1
              if: always() && github.repository == 'keycloak/keycloak'
              with:
                api_key: ${{ secrets.FORESIGHT_API_KEY }}
                test_format: JUNIT
                test_framework: JUNIT
                test_path: 'quarkus/tests/integration/target/surefire-reports/*.xml'
            - name: Quarkus test reports
              uses: actions/upload-artifact@v3
              if: failure()
              with:
                name: reports-quarkus-tests
                retention-days: 14
                path: reports-quarkus-tests.zip
                if-no-files-found: ignore

# NOTE: WebAuthn tests can be enabled once the issue #12621 is resolved
#
#  webauthn-test:
#    name: WebAuthn Tests
#    needs: build
#    runs-on: ubuntu-latest
#    steps:
#      - uses: actions/checkout@v2
#        with:
#          fetch-depth: 2
#
#      - name: Check whether this phase should run
#        run: echo "GIT_DIFF=$[ $( git diff --name-only HEAD^ | egrep -ic 'webauthn|passwordless' ) ]" >> $GITHUB_ENV
#
#      - uses: actions/setup-java@v1
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        with:
#          java-version: ${{ env.DEFAULT_JDK_VERSION }}
#
#      - name: Update maven settings
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        run: mkdir -p ~/.m2 ; cp .github/settings.xml ~/.m2/
#
#      - name: Cache Maven packages
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        uses: actions/cache@v2
#        with:
#          path: ~/.m2/repository
#          key: cache-1-${{ runner.os }}-m2-${{ hashFiles('**/pom.xml') }}
#          restore-keys: cache-1-${{ runner.os }}-m2
#
#      - name: Cleanup org.keycloak artifacts
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        run: rm -rf ~/.m2/repository/org/keycloak >/dev/null || true
#
#      - name: Download built keycloak
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        id: download-keycloak
#        uses: actions/download-artifact@v2
#        with:
#          path: ~/.m2/repository/org/keycloak/
#          name: keycloak-artifacts.zip
#
#      - name: Run WebAuthn tests
#        if: ${{ github.event_name != 'pull_request' || env.GIT_DIFF != 0 }}
#        run: |
#          mvn clean install -nsu -B -Dbrowser=chrome -Pwebauthn -f testsuite/integration-arquillian/tests/other/pom.xml -Dtest=org.keycloak.testsuite.webauthn.**.*Test | misc/log/trimmer.sh
#
#          TEST_RESULT=${PIPESTATUS[0]}
#          find . -path '*/target/surefire-reports/*.xml' | zip -q reports-webauthn-tests.zip -@
#          exit $TEST_RESULT
#
#      - name: WebAuthn test reports
#        uses: actions/upload-artifact@v2
#        if: failure()
#        with:
#          name: reports-webauthn-tests
#          retention-days: 14
#          path: reports-webauthn-tests.zip
#          if-no-files-found: ignore
`
	assert.Equal(t, expectedData, string(devData))
}
