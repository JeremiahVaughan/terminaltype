package config

import (
    "io"
    "fmt"
    "context"

    "github.com/aws/aws-sdk-go-v2/aws"                                   
    s3_config "github.com/aws/aws-sdk-go-v2/config"                                
    "github.com/aws/aws-sdk-go-v2/service/s3"                            
)

func fetchConfigFromS3(ctx context.Context, serviceName string) ([]byte, error) {                                                     
   cfg, err := s3_config.LoadDefaultConfig(ctx) 
   if err != nil {                                                   
       return nil, fmt.Errorf("error, unable to load SDK config, " +        
           "you may need to configure your AWS credentials. Error: %v", err)                                                                  
   }                                                                 
                                                                     
   svc := s3.NewFromConfig(cfg)                                      

   bucketName := "config-bunker"
   configFile := fmt.Sprintf("%s/config.json", serviceName)
   objectInput := &s3.GetObjectInput{                                
       Bucket: aws.String(bucketName),                                   
       Key: aws.String(configFile),                                         
   }                                                                 
                                                                     
   result, err := svc.GetObject(ctx, objectInput)                    
   if err != nil {                                                   
       return nil, fmt.Errorf("error, failed to get object %s from bucket %s. Error: %v", configFile, bucketName, err)                                            
   }                                                                 
   defer result.Body.Close()                                         
                                                                     
   body, err := io.ReadAll(result.Body)                          
   if err != nil {                                                   
       return nil, fmt.Errorf("error, failed to read object body. Error: %v", err) 
   }                                                                 
                                                                     
   return body, nil                                                  
}                                                                     
