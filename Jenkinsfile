pipeline {
    agent any

    triggers {
        githubPush()
    }

    environment {
        IMG = "cloudfgd-app:${BUILD_NUMBER}"
    }

    stages {
        stage('Build') {
            steps {
                sh 'eval $(minikube docker-env) && docker build -t $IMG -t cloudfgd-app:latest srcs/requirements/app'
            }
        }
        stage('Deploy') {
            steps {
                sh 'kubectl apply -f k8s/'
                sh 'kubectl set image deployment/cloudfgd app=$IMG'
                sh 'kubectl rollout status deployment/cloudfgd --timeout=180s'
            }
        }
        stage('Verify') {
            steps {
                sh 'kubectl get pods -l app=cloudfgd -o wide'
                sh 'kubectl get svc cloudfgd'
            }
        }
    }

    post {
        failure {
            sh 'kubectl describe deployment cloudfgd || true'
            sh 'kubectl get events --sort-by=.lastTimestamp | tail -20 || true'
        }
    }
}
