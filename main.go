package main

import "fmt"

var credentials = Credentials{
	Region:          "us-east-2",
	Bucket:          "kikori-staging",
	AccessKeyID:     "AKIAQAOIKVCA7SEZU6PR",
	SecretAccessKey: "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",
}

var policyOptions = PolicyOptions{
	ExpiryMinutes: 15,
	MaxFileSize:   20 * 1024 * 1024,
}

func main() {

	var config = Config{
		Credentials:   credentials,
		PolicyOptions: policyOptions,
	}

	var p = config.GenerateSignature()

	fmt.Println("URL", p.URL)
	fmt.Println("ACL", p.ACL)
	fmt.Println("Algorithm", p.Algorithm)
	fmt.Println("Credential", p.Credential)
	fmt.Println("Date", p.Date)
	fmt.Println("Policy", p.Policy)
	fmt.Println("Signature", p.Signature)
}
