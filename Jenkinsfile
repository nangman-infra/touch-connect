def WEBHOOK_TRIGGER_TOKEN_CREDENTIAL_ID = 'GITHUB_WEBHOOK_TRIGGER_TOKEN'
def SPOT_AGENT_LABEL = 'spot-agent'
def ONPREM_WATCHTOWER_TRIGGER_AGENT_LABEL = 'onprem-watchtower-trigger'
def PRIMARY_BUILDX_BUILDER = 'default'
def FALLBACK_BUILDX_BUILDER = 'multiarch-builder'
def REPO_SLUG = 'nangman-infra/touch-connect'
def MAIN_BRANCH_REF = 'refs/heads/main'
def DEFAULT_REPO_HTTP_URL = 'https://github.com/nangman-infra/touch-connect.git'

pipeline {
    agent none

    parameters {
        booleanParam(
            name: 'FORCE_DEPLOY',
            defaultValue: false,
            description: '서버/컨트롤 변경이 없어도 이미지 빌드, 푸시, 배포를 강제로 실행합니다.'
        )
    }

    triggers {
        GenericTrigger(
            genericVariables: [
                [key: 'GIT_REF', value: '$.ref', defaultValue: ''],
                [key: 'REPO_URL', value: '$.repository.clone_url', defaultValue: ''],
                [key: 'BEFORE_SHA', value: '$.before', defaultValue: ''],
                [key: 'AFTER_SHA', value: '$.after', defaultValue: '']
            ],
            tokenCredentialId: WEBHOOK_TRIGGER_TOKEN_CREDENTIAL_ID,
            causeString: 'touch-connect main push detected',
            regexpFilterText: '$REPO_URL $GIT_REF',
            regexpFilterExpression: ".*${REPO_SLUG}.* ${MAIN_BRANCH_REF}",
            printContributedVariables: true,
            printPostContent: true
        )
    }

    environment {
        HARBOR_URL = 'harbor.nangman.cloud'
        HARBOR_PROJECT = 'library'
        HARBOR_CREDS_ID = 'NANGMAN_HARBOR_ROBOT_ACCOUNT'

        SERVER_IMAGE_NAME = 'touch-connect-server'
        SERVER_IMAGE_REPO = "${HARBOR_URL}/${HARBOR_PROJECT}/${SERVER_IMAGE_NAME}"
        SERVER_IMAGE_CACHE = "${SERVER_IMAGE_REPO}:buildcache"
        SERVER_IMAGE_LATEST = "${SERVER_IMAGE_REPO}:latest"

        CONTROL_IMAGE_NAME = 'touch-connect-control'
        CONTROL_IMAGE_REPO = "${HARBOR_URL}/${HARBOR_PROJECT}/${CONTROL_IMAGE_NAME}"
        CONTROL_IMAGE_CACHE = "${CONTROL_IMAGE_REPO}:buildcache"
        CONTROL_IMAGE_LATEST = "${CONTROL_IMAGE_REPO}:latest"

        WATCHTOWER_URL = 'http://172.16.0.34:18081'
        WATCHTOWER_TOKEN = credentials('nangman-infra-touch-connect-watchtower-token')
        SERVER_HEALTH_URL = 'http://172.16.0.34:8080/healthz'
        CONTROL_READY_URL = 'http://172.16.0.34:8081/readyz'
        SERVER_HEALTH_EXPECTED_STATUS = 'ok'
        CONTROL_READY_EXPECTED_STATUS = 'ready'
        SERVER_HEALTH_EXPECTED_COMPONENT = 'tc-server'
        CONTROL_READY_EXPECTED_COMPONENT = 'tc-control'
        DEPLOY_TIMEOUT_SECONDS = '180'

        SONARQUBE_INSTALLATION = 'sonarqube'
        SONAR_SCANNER_TOOL = 'SonarScanner'
        SONAR_PROJECT_KEY = 'touch-connect-server'
        SONAR_PROJECT_NAME = 'touch-connect-server'
        GO_COVERAGE_REPORT = 'coverage.out'
        CI = 'true'

        DOCKER_BUILDKIT = '1'
        DOCKER_CLI_EXPERIMENTAL = 'enabled'
        PLATFORMS = 'linux/amd64,linux/arm64'
    }

    options {
        skipDefaultCheckout(true)
        disableConcurrentBuilds()
        buildDiscarder(logRotator(numToKeepStr: '10'))
        timeout(time: 60, unit: 'MINUTES')
        timestamps()
        ansiColor('xterm')
    }

    stages {
        stage('Validate And Build On Spot') {
            agent { label SPOT_AGENT_LABEL }
            stages {
                stage('Checkout') {
                    steps {
                        checkout scm
                    }
                }

                stage('Initialize') {
                    steps {
                        script {
                            env.FULL_SHA = sh(script: 'git rev-parse HEAD', returnStdout: true).trim()
                            env.SHORT_SHA = sh(script: 'git rev-parse --short=12 HEAD', returnStdout: true).trim()
                            env.EXACT_GIT_TAG = sh(
                                script: 'git fetch --tags --force >/dev/null 2>&1 || true; git tag --points-at HEAD | head -n 1',
                                returnStdout: true
                            ).trim()
                            env.BUILD_TIMESTAMP = sh(
                                script: 'date -u +%Y-%m-%dT%H:%M:%SZ',
                                returnStdout: true
                            ).trim()
                            env.BUILD_REF = env.GIT_REF ?: MAIN_BRANCH_REF
                            env.REPO_HTTP_URL = env.REPO_URL?.trim()
                                ? env.REPO_URL.trim()
                                : DEFAULT_REPO_HTTP_URL

                            def hasBeforeSha = env.BEFORE_SHA?.trim() && sh(
                                script: "git cat-file -e ${env.BEFORE_SHA}^{commit} >/dev/null 2>&1",
                                returnStatus: true
                            ) == 0
                            def hasAfterSha = env.AFTER_SHA?.trim() && sh(
                                script: "git cat-file -e ${env.AFTER_SHA}^{commit} >/dev/null 2>&1",
                                returnStatus: true
                            ) == 0
                            def diffLabel
                            def changedFilesText

                            if (hasBeforeSha && hasAfterSha) {
                                diffLabel = "${env.BEFORE_SHA.take(12)}..${env.AFTER_SHA.take(12)}"
                                changedFilesText = sh(
                                    script: "git diff --name-only ${env.BEFORE_SHA} ${env.AFTER_SHA}",
                                    returnStdout: true
                                ).trim()
                            } else if (sh(script: 'git rev-parse HEAD^ >/dev/null 2>&1', returnStatus: true) == 0) {
                                diffLabel = 'HEAD^..HEAD'
                                changedFilesText = sh(
                                    script: 'git diff --name-only HEAD^ HEAD',
                                    returnStdout: true
                                ).trim()
                            } else {
                                diffLabel = 'full-tree'
                                changedFilesText = sh(
                                    script: 'git ls-tree --name-only -r HEAD',
                                    returnStdout: true
                                ).trim()
                            }

                            def changedFiles = changedFilesText ? changedFilesText.readLines() : []
                            def deployExactPaths = [
                                'Dockerfile',
                                '.dockerignore',
                                'go.mod',
                                'go.sum',
                                'docker-compose.dev.yml',
                                'Jenkinsfile'
                            ] as Set
                            def deployPrefixes = [
                                'internal/',
                                'tc-server/',
                                'tc-control/',
                                'deploy/onprem/'
                            ]
                            def serverChanged = diffLabel == 'full-tree' || changedFiles.any { path ->
                                deployExactPaths.contains(path) || deployPrefixes.any { prefix -> path.startsWith(prefix) }
                            }
                            def forceDeploy = params.FORCE_DEPLOY == true
                            def imageBuildRequired = forceDeploy || serverChanged

                            env.SERVER_CHANGED = serverChanged ? 'true' : 'false'
                            env.FORCE_DEPLOY = forceDeploy ? 'true' : 'false'
                            env.IMAGE_BUILD_REQUIRED = imageBuildRequired ? 'true' : 'false'
                            env.DEPLOY_REQUIRED = imageBuildRequired ? 'true' : 'false'
                            env.SERVER_SHA_TAG = "${env.SERVER_IMAGE_REPO}:sha-${env.SHORT_SHA}"
                            env.CONTROL_SHA_TAG = "${env.CONTROL_IMAGE_REPO}:sha-${env.SHORT_SHA}"
                            env.FAILURE_CATEGORY = 'build'
                            env.FAILURE_STAGE = 'Initialize'
                            env.FAILURE_REASON = '빌드 단계에서 실패했습니다.'

                            currentBuild.displayName = "#${env.BUILD_NUMBER} ${env.SHORT_SHA}"
                            currentBuild.description = (
                                env.EXACT_GIT_TAG
                                    ? "main -> ${env.EXACT_GIT_TAG}"
                                    : "main -> sha-${env.SHORT_SHA}"
                            ) + " | server=${env.SERVER_CHANGED}"

                            echo "Repository: ${env.REPO_HTTP_URL}"
                            echo "Branch ref: ${env.BUILD_REF}"
                            echo "Diff scope: ${diffLabel}"
                            echo "Changed files: ${changedFiles ? changedFiles.join(', ') : '(none)'}"
                            echo "Server image repository: ${env.SERVER_IMAGE_REPO}"
                            echo "Control image repository: ${env.CONTROL_IMAGE_REPO}"
                            echo "Force deploy requested: ${env.FORCE_DEPLOY}"
                            echo "Server/control deploy surface changed: ${env.SERVER_CHANGED}"
                            echo "Image build required: ${env.IMAGE_BUILD_REQUIRED}"
                            echo "Image tags: latest, sha-${env.SHORT_SHA}${env.EXACT_GIT_TAG ? ", ${env.EXACT_GIT_TAG}" : ''}"

                            if (env.FORCE_DEPLOY == 'true') {
                                echo 'FORCE_DEPLOY=true 이므로 변경 파일과 관계없이 빌드, 푸시, 배포를 진행합니다.'
                            } else if (env.DEPLOY_REQUIRED != 'true') {
                                echo 'No deployable server/control changes detected; image push and deploy stages will be skipped.'
                            }
                        }
                    }
                }

                stage('Notify Build Start') {
                    steps {
                        script {
                            def startMessage = ":hourglass_flowing_sand: 빌드를 시작합니다.\n프로젝트: ${env.JOB_NAME} #${env.BUILD_NUMBER}\n브랜치: ${env.BUILD_REF}\n태그: sha-${env.SHORT_SHA}"

                            if (env.FORCE_DEPLOY == 'true') {
                                startMessage += "\n실행 방식: 강제 배포"
                            } else if (env.DEPLOY_REQUIRED == 'true') {
                                startMessage += "\n실행 방식: 변경 감지 배포"
                            } else {
                                startMessage += "\n실행 방식: 품질 검증 전용"
                            }

                            try {
                                mattermostSend(
                                    color: '#439FE0',
                                    message: startMessage
                                )
                            } catch (err) {
                                echo "Mattermost start notification failed: ${err.getMessage()}"
                            }
                        }
                    }
                }

                stage('Validate Go And Contracts') {
                    options {
                        timeout(time: 20, unit: 'MINUTES')
                    }
                    steps {
                        script {
                            env.FAILURE_CATEGORY = 'quality'
                            env.FAILURE_STAGE = 'Validate Go And Contracts'
                            env.FAILURE_REASON = 'Go 테스트 또는 touch-connect 계약 검증에 실패했습니다.'
                        }
                        sh '''
                            set -eu
                            host_uid=$(id -u)
                            host_gid=$(id -g)

                            docker run --rm \
                                -e GO_COVERAGE_REPORT="$GO_COVERAGE_REPORT" \
                                -e HOST_UID="$host_uid" \
                                -e HOST_GID="$host_gid" \
                                -v "$PWD:/workspace" \
                                -w /workspace \
                                golang:1.25-alpine \
                                sh -lc '
                                    set -eu
                                    export PATH=/usr/local/go/bin:$PATH
                                    apk add --no-cache python3
                                    go version
                                    python3 --version
                                    go test ./... -coverprofile="$GO_COVERAGE_REPORT"
                                    python3 scripts/validate_docs.py
                                    test -f "$GO_COVERAGE_REPORT"
                                    chown "$HOST_UID:$HOST_GID" "$GO_COVERAGE_REPORT"
                                '
                        '''
                    }
                }

                stage('SonarQube Analysis') {
                    steps {
                        script {
                            env.FAILURE_CATEGORY = 'sonar'
                            env.FAILURE_STAGE = 'SonarQube Analysis'
                            env.FAILURE_REASON = 'SonarQube 분석에 실패해 배포가 중단되었습니다.'
                            def scannerHome = tool env.SONAR_SCANNER_TOOL

                            writeFile(
                                file: 'sonar-project.properties',
                                text: """
                                    sonar.projectKey=${env.SONAR_PROJECT_KEY}
                                    sonar.projectName=${env.SONAR_PROJECT_NAME}
                                    sonar.projectVersion=sha-${env.SHORT_SHA}
                                    sonar.projectBaseDir=.
                                    sonar.sourceEncoding=UTF-8
                                    sonar.scm.revision=${env.FULL_SHA}
                                    sonar.sources=internal,tc-server,tc-control
                                    sonar.tests=internal,tc-server,tc-control,tests
                                    sonar.test.inclusions=**/*_test.go
                                    sonar.exclusions=**/*_test.go,**/vendor/**,**/.touch-connect/**,**/coverage/**,**/node_modules/**,tc-worker/**,tcctl/**,examples/**,deploy/**
                                    sonar.go.coverage.reportPaths=${env.GO_COVERAGE_REPORT}
                                """.stripIndent().trim() + '\n'
                            )

                            withSonarQubeEnv(env.SONARQUBE_INSTALLATION) {
                                sh "\"${scannerHome}/bin/sonar-scanner\" -Dproject.settings=sonar-project.properties"
                            }
                        }
                    }
                }

                stage('Quality Gate') {
                    steps {
                        script {
                            env.FAILURE_CATEGORY = 'sonar'
                            env.FAILURE_STAGE = 'Quality Gate'
                            env.FAILURE_REASON = 'SonarQube 품질 기준을 통과하지 못해 배포가 중단되었습니다.'
                        }
                        timeout(time: 30, unit: 'MINUTES') {
                            waitForQualityGate abortPipeline: true
                        }
                    }
                }

                stage('Setup Buildx') {
                    when {
                        expression { env.IMAGE_BUILD_REQUIRED == 'true' }
                    }
                    steps {
                        script {
                            env.FAILURE_CATEGORY = 'build'
                            env.FAILURE_STAGE = 'Setup Buildx'
                            env.FAILURE_REASON = '멀티아키 빌드 환경 준비에 실패했습니다.'
                        }
                        sh """
                            docker buildx version

                            if docker buildx inspect ${PRIMARY_BUILDX_BUILDER} >/dev/null 2>&1; then
                                docker buildx use ${PRIMARY_BUILDX_BUILDER}
                                docker buildx inspect ${PRIMARY_BUILDX_BUILDER} --bootstrap
                            else
                                if docker buildx inspect ${FALLBACK_BUILDX_BUILDER} >/dev/null 2>&1; then
                                    docker buildx use ${FALLBACK_BUILDX_BUILDER}
                                else
                                    docker buildx create --name ${FALLBACK_BUILDX_BUILDER} --use --platform "\$PLATFORMS"
                                fi

                                docker buildx inspect ${FALLBACK_BUILDX_BUILDER} --bootstrap
                            fi
                        """
                    }
                }

                stage('Docker Build & Push Server') {
                    when {
                        expression { env.IMAGE_BUILD_REQUIRED == 'true' }
                    }
                    options {
                        timeout(time: 45, unit: 'MINUTES')
                    }
                    steps {
                        script {
                            env.FAILURE_CATEGORY = 'build'
                            env.FAILURE_STAGE = 'Docker Build & Push Server'
                            env.FAILURE_REASON = 'tc-server 이미지 빌드 또는 푸시에 실패했습니다.'
                            buildAndPushTouchConnectImage(
                                'tc-server',
                                env.SERVER_IMAGE_REPO,
                                env.SERVER_IMAGE_CACHE,
                                env.SERVER_IMAGE_LATEST,
                                env.SERVER_SHA_TAG
                            )
                        }
                    }
                }

                stage('Docker Build & Push Control') {
                    when {
                        expression { env.IMAGE_BUILD_REQUIRED == 'true' }
                    }
                    options {
                        timeout(time: 45, unit: 'MINUTES')
                    }
                    steps {
                        script {
                            env.FAILURE_CATEGORY = 'build'
                            env.FAILURE_STAGE = 'Docker Build & Push Control'
                            env.FAILURE_REASON = 'tc-control 이미지 빌드 또는 푸시에 실패했습니다.'
                            buildAndPushTouchConnectImage(
                                'tc-control',
                                env.CONTROL_IMAGE_REPO,
                                env.CONTROL_IMAGE_CACHE,
                                env.CONTROL_IMAGE_LATEST,
                                env.CONTROL_SHA_TAG
                            )
                        }
                    }
                }

                stage('Verify Images') {
                    when {
                        expression { env.IMAGE_BUILD_REQUIRED == 'true' }
                    }
                    steps {
                        script {
                            env.FAILURE_CATEGORY = 'build'
                            env.FAILURE_STAGE = 'Verify Images'
                            env.FAILURE_REASON = '푸시한 이미지 검증에 실패했습니다.'
                            withCredentials([
                                usernamePassword(
                                    credentialsId: env.HARBOR_CREDS_ID,
                                    usernameVariable: 'HARBOR_USERNAME',
                                    passwordVariable: 'HARBOR_PASSWORD'
                                )
                            ]) {
                                sh '''
                                    set -eu
                                    echo "$HARBOR_PASSWORD" | docker login $HARBOR_URL -u "$HARBOR_USERNAME" --password-stdin
                                '''

                                try {
                                    sh '''
                                        echo "Inspecting server latest manifest"
                                        docker buildx imagetools inspect "$SERVER_IMAGE_LATEST"
                                        echo "Inspecting server sha manifest"
                                        docker buildx imagetools inspect "$SERVER_SHA_TAG"
                                        echo "Inspecting control latest manifest"
                                        docker buildx imagetools inspect "$CONTROL_IMAGE_LATEST"
                                        echo "Inspecting control sha manifest"
                                        docker buildx imagetools inspect "$CONTROL_SHA_TAG"
                                    '''

                                    if (env.EXACT_GIT_TAG) {
                                        sh '''
                                            echo "Inspecting server git tag manifest"
                                            docker buildx imagetools inspect "$SERVER_IMAGE_REPO:$EXACT_GIT_TAG"
                                            echo "Inspecting control git tag manifest"
                                            docker buildx imagetools inspect "$CONTROL_IMAGE_REPO:$EXACT_GIT_TAG"
                                        '''
                                    }
                                } finally {
                                    sh 'docker logout $HARBOR_URL'
                                }
                            }
                        }
                    }
                }
            }
        }

        stage('Deploy On Onprem') {
            agent { label ONPREM_WATCHTOWER_TRIGGER_AGENT_LABEL }
            when {
                expression { env.DEPLOY_REQUIRED == 'true' }
            }
            stages {
                stage('Trigger Watchtower') {
                    steps {
                        script {
                            env.FAILURE_CATEGORY = 'deploy'
                            env.FAILURE_STAGE = 'Trigger Watchtower'
                            env.FAILURE_REASON = '배포 트리거 호출에 실패했습니다.'
                        }
                        sh '''
                            set -eu

                            response=$(curl -sS -w "\\n%{http_code}" \
                                -H "Authorization: Bearer $WATCHTOWER_TOKEN" \
                                "$WATCHTOWER_URL/v1/update")

                            http_code=$(echo "$response" | tail -n1)
                            body=$(echo "$response" | sed '$d')

                            if [ "$http_code" -eq 200 ]; then
                                echo "Watchtower update triggered successfully"
                                echo "Response: $body"
                            else
                                echo "Failed to trigger Watchtower update"
                                echo "HTTP Code: $http_code"
                                echo "Response: $body"
                                exit 1
                            fi
                        '''
                    }
                }

                stage('Verify Deployment') {
                    steps {
                        script {
                            env.FAILURE_CATEGORY = 'deploy'
                            env.FAILURE_STAGE = 'Verify Deployment'
                            env.FAILURE_REASON = 'tc-server/tc-control 배포 후 상태 검증에 실패했습니다.'
                        }
                        sh '''
                            set -eu
                            deadline=$(( $(date +%s) + $DEPLOY_TIMEOUT_SECONDS ))

                            while [ "$(date +%s)" -lt "$deadline" ]; do
                                server_body=$(curl -fsS "$SERVER_HEALTH_URL" || true)
                                control_body=$(curl -fsS "$CONTROL_READY_URL" || true)

                                if [ -n "$server_body" ]; then
                                    echo "Server health response: $server_body"
                                fi

                                if [ -n "$control_body" ]; then
                                    echo "Control ready response: $control_body"
                                fi

                                if echo "$server_body" | grep -Eq '"status"[[:space:]]*:[[:space:]]*"'"$SERVER_HEALTH_EXPECTED_STATUS"'"' \
                                    && echo "$server_body" | grep -q "$SERVER_HEALTH_EXPECTED_COMPONENT" \
                                    && echo "$control_body" | grep -Eq '"status"[[:space:]]*:[[:space:]]*"'"$CONTROL_READY_EXPECTED_STATUS"'"' \
                                    && echo "$control_body" | grep -q "$CONTROL_READY_EXPECTED_COMPONENT"; then
                                    echo "Deployment verified: $SERVER_HEALTH_URL and $CONTROL_READY_URL"
                                    exit 0
                                fi

                                sleep 5
                            done

                            echo "Deployment verification timed out after ${DEPLOY_TIMEOUT_SECONDS}s"
                            exit 1
                        '''
                    }
                }
            }
        }
    }

    post {
        success {
            script {
                def successMessage = env.DEPLOY_REQUIRED == 'true'
                    ? ":tada: touch-connect 빌드 성공! 배포가 완료되었습니다.\n프로젝트: ${env.JOB_NAME} #${env.BUILD_NUMBER}\n바로가기: ${env.BUILD_URL}"
                    : ":white_check_mark: touch-connect 빌드와 품질 검증이 성공했습니다. 서버/컨트롤 배포 대상 변경이 없어 이미지 푸시와 배포는 생략되었습니다.\n프로젝트: ${env.JOB_NAME} #${env.BUILD_NUMBER}\n바로가기: ${env.BUILD_URL}"

                mattermostSend(
                    color: 'good',
                    message: successMessage
                )
            }
        }

        failure {
            script {
                def failureHeadline

                if (env.FAILURE_CATEGORY == 'sonar') {
                    failureHeadline = env.DEPLOY_REQUIRED == 'true'
                        ? ":warning: SonarQube 품질 검증 실패로 touch-connect 배포가 중단되었습니다."
                        : ":warning: SonarQube 품질 검증에 실패했습니다."
                } else if (env.FAILURE_CATEGORY == 'quality') {
                    failureHeadline = env.DEPLOY_REQUIRED == 'true'
                        ? ":warning: Go 테스트 또는 계약 검증 실패로 touch-connect 배포가 중단되었습니다."
                        : ":warning: Go 테스트 또는 계약 검증에 실패했습니다."
                } else if (env.FAILURE_CATEGORY == 'deploy') {
                    failureHeadline = ":rotating_light: 이미지 빌드는 완료됐지만 touch-connect 배포 단계에서 실패했습니다."
                } else {
                    failureHeadline = ":rotating_light: touch-connect 빌드 실패... 로그를 확인해주세요."
                }

                mattermostSend(
                    color: 'danger',
                    message: "${failureHeadline}\n실패 단계: ${env.FAILURE_STAGE}\n사유: ${env.FAILURE_REASON}\n프로젝트: ${env.JOB_NAME} #${env.BUILD_NUMBER}\n바로가기: ${env.BUILD_URL}"
                )
            }
        }

        always {
            script {
                echo "빌드 완료. Buildx는 이미지를 직접 푸시하므로 로컬 정리가 불필요합니다."
            }
        }
    }
}

def buildAndPushTouchConnectImage(String target, String imageRepo, String imageCache, String imageLatest, String imageShaTag) {
    withCredentials([
        usernamePassword(
            credentialsId: env.HARBOR_CREDS_ID,
            usernameVariable: 'HARBOR_USERNAME',
            passwordVariable: 'HARBOR_PASSWORD'
        )
    ]) {
        sh """
            set -eu
            echo "\$HARBOR_PASSWORD" | docker login ${env.HARBOR_URL} -u "\$HARBOR_USERNAME" --password-stdin
        """

        try {
            def cacheFromArg = sh(
                script: "docker buildx imagetools inspect ${imageCache} >/dev/null 2>&1",
                returnStatus: true
            ) == 0
                ? "--cache-from type=registry,ref=${imageCache}"
                : ""
            def tagArgs = [
                "--tag ${imageLatest}",
                "--tag ${imageShaTag}"
            ]

            if (env.EXACT_GIT_TAG) {
                tagArgs << "--tag ${imageRepo}:${env.EXACT_GIT_TAG}"
            }

            def imageVersion = env.EXACT_GIT_TAG ?: "sha-${env.SHORT_SHA}"
            def buildArgs = [
                "--platform ${env.PLATFORMS}",
                "--file Dockerfile",
                "--target ${target}",
                "--label org.opencontainers.image.created=${env.BUILD_TIMESTAMP}",
                "--label org.opencontainers.image.revision=${env.GIT_COMMIT}",
                "--label org.opencontainers.image.source=${env.REPO_HTTP_URL}",
                "--label org.opencontainers.image.version=${imageVersion}",
                "--pull"
            ] + tagArgs

            if (cacheFromArg) {
                buildArgs << cacheFromArg
            }

            buildArgs += [
                "--cache-to type=registry,ref=${imageCache},mode=max",
                "--push",
                "--progress=plain",
                "."
            ]

            sh """
                docker buildx build \\
                    ${buildArgs.join(' \\\n                    ')}
            """
        } finally {
            sh 'docker logout $HARBOR_URL'
        }
    }
}
