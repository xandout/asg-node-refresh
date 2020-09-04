package main

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	cs                 *kubernetes.Clientset
	namespace          string
	configMapName      string
	job                string
	awsAsgName         string
	autoScalingSession *autoscaling.AutoScaling
	refreshID          string
	instanceWarmup     int64
)

func getCM(namespace string, name string) *v1.ConfigMap {
	cm, err := cs.CoreV1().ConfigMaps(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}
	return cm
}

func updateCM(namespace string, cm *v1.ConfigMap) *v1.ConfigMap {
	cm, err := cs.CoreV1().ConfigMaps(namespace).Update(cm)
	if err != nil {
		log.Fatal(err)
	}
	return cm
}

func startRefresh(instanceWarmup, minHealthyPercentage int64) (*autoscaling.StartInstanceRefreshOutput, error) {
	autoScalingSession = autoscaling.New(session.New())
	input := &autoscaling.StartInstanceRefreshInput{
		AutoScalingGroupName: aws.String(awsAsgName),
		Preferences: &autoscaling.RefreshPreferences{
			InstanceWarmup:       aws.Int64(instanceWarmup),
			MinHealthyPercentage: aws.Int64(minHealthyPercentage),
		},
	}

	return autoScalingSession.StartInstanceRefresh(input)
}

func describeRefresh(refreshID *string) (*autoscaling.DescribeInstanceRefreshesOutput, error) {
	autoScalingSession = autoscaling.New(session.New())

	input := &autoscaling.DescribeInstanceRefreshesInput{
		AutoScalingGroupName: aws.String(awsAsgName),
		InstanceRefreshIds:   []*string{refreshID},
	}

	return autoScalingSession.DescribeInstanceRefreshes(input)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func init() {
	namespace = os.Getenv("NAMESPACE")
	configMapName = os.Getenv("CONFIGMAP_NAME")
	job = os.Getenv("JOB")
	awsAsgName = os.Getenv("AWS_ASG_NAME")
	iw, _ := strconv.Atoi(getEnv("INSTANCE_WARMUP", "300"))
	instanceWarmup = int64(iw)
	// creates the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	// creates the clientset
	cs, err = kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
}

func main() {

	cm := getCM(namespace, configMapName)
	if cm.Data == nil {
		log.Println("Creating refresh request with AWS...")
		refreshOutput, err := startRefresh(instanceWarmup, 90)
		if err != nil {
			log.Fatalf("Error occured while requesting an instance refresh: %v", err)
		}

		cm.Data = map[string]string{}
		cm.Data["job"] = job
		cm.Data["refresh-id"] = *refreshOutput.InstanceRefreshId
		refreshID = *refreshOutput.InstanceRefreshId
		cm = updateCM(namespace, cm)
		log.Printf("Created instance refresh: %s\n", refreshID)
		log.Println("Begin polling the refresh every 10 seconds...")
	}
	if cm.Data["job"] == job {
		refreshID = cm.Data["refresh-id"]
		log.Printf("Continuing with refresh-id %s", refreshID)
	loop:
		for {
			descOutput, err := describeRefresh(&refreshID)
			if err != nil {
				log.Fatalf("Error occurred describing refresh %s: %v", refreshID, err)
				break
			}
			out := descOutput.InstanceRefreshes[0]
			switch s := *out.Status; s {
			// "Pending"
			// "InProgress"
			// "Successful"
			// "Failed"
			// "Cancelling"
			// "Cancelled"
			case "Pending", "InProgress", "Cancelling":
				log.Printf("Refresh %s is %s", refreshID, s)
			case "Cancelled", "Failed":
				log.Fatalf("Refresh %s is %s. Reason %s", refreshID, s, *out.StatusReason)
			case "Successful":
				log.Printf("Refresh %s was successful!", refreshID)
				break loop
			default:
				log.Fatalf("Refresh %s is %s and we don't know what that means.", refreshID, s)
			}
			time.Sleep(10 * time.Second)
		}
		delete(cm.Data, "job")
		delete(cm.Data, "refresh-id")
		updateCM(namespace, cm)
	} else {
		log.Printf("Job name in ConfigMap named %s is %s, which is not this job.\n", configMapName, cm.Data["job"])
		log.Println("Aborting...")
		log.Printf("If no other %s jobs are running, delete the 'job' & 'refresh-id' keys from the ConfigMap named %s", job, configMapName)
		os.Exit(1)
	}
}
