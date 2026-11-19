pipeline {
    agent any

    triggers {
        githubPush()
    }

    options {
        disableConcurrentBuilds()
        timeout(time: 20, unit: 'MINUTES')
    }

    stages {
        stage('Preparar') {
            steps {
                script {
                    def rama = env.GIT_BRANCH ?: ''
                    env.NS = rama.endsWith('main') ? 'produccion' : 'pruebas'
                    env.IMG = "cloudfgd-app:${env.NS}-${env.BUILD_NUMBER}"
                    echo "Rama: ${rama} -> entorno: ${env.NS} -> imagen: ${env.IMG}"
                }
            }
        }
        stage('Test') {
            steps {
                sh 'eval $(minikube docker-env) && docker build --target test -t cloudfgd-app:test srcs/requirements/app'
            }
        }
        stage('Build') {
            steps {
                sh 'eval $(minikube docker-env) && docker build -t $IMG srcs/requirements/app'
            }
        }
        stage('Deploy') {
            steps {
                sh 'kubectl create namespace $NS --dry-run=client -o yaml | kubectl apply -f -'
                sh 'sed "s|cloudfgd-app:latest|$IMG|" k8s/deployment.yaml | kubectl apply -n $NS -f -'
                sh 'kubectl apply -n $NS -f k8s/service.yaml'
                sh 'kubectl rollout status deployment/cloudfgd -n $NS --timeout=180s'
            }
        }
        stage('Verify') {
            steps {
                sh 'kubectl get pods -n $NS -l app=cloudfgd -o wide'
                sh 'kubectl get svc cloudfgd -n $NS'
            }
        }
    }

    post {
        failure {
            sh 'kubectl describe deployment cloudfgd -n $NS || true'
            sh 'kubectl get events -n $NS --sort-by=.lastTimestamp | tail -20 || true'
        }
    }
}
