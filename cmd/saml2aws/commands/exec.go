package commands

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/pkg/errors"
	"github.com/versent/saml2aws/pkg/awsconfig"
	"github.com/versent/saml2aws/pkg/flags"
	"github.com/versent/saml2aws/pkg/shell"
)

// Exec execute the supplied command after seeding the environment
func Exec(execFlags *flags.LoginExecFlags, cmdline []string) error {

	if len(cmdline) < 1 {
		return fmt.Errorf("Command to execute required")
	}

	account, err := buildIdpAccount(execFlags)
	if err != nil {
		return errors.Wrap(err, "error building login details")
	}

	sharedCreds := awsconfig.NewSharedCredentials(account.Profile)

	// this checks if the credentials file has been created yet
	// can only really be triggered if saml2aws exec is run on a new
	// system prior to creating $HOME/.aws
	exist, err := sharedCreds.CredsExists()
	if err != nil {
		return errors.Wrap(err, "error loading credentials")
	}
	if !exist {
		fmt.Println("unable to load credentials, login required to create them")
		return nil
	}

	awsCreds, err := sharedCreds.Load()
	if err != nil {
		return errors.Wrap(err, "error loading credentials")
	}

	if awsCreds.Expires.Sub(time.Now()) < 0 {
		return errors.New("error aws credentials have expired")
	}

	ok, err := checkToken(account.Profile)
	if err != nil {
		return errors.Wrap(err, "error validating token")
	}

	if !ok {
		err = Login(execFlags)
	}
	if err != nil {
		return errors.Wrap(err, "error logging in")
	}

	return shell.ExecShellCmd(cmdline, shell.BuildEnvVars(awsCreds, account, execFlags))
}

func checkToken(profile string) (bool, error) {
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: profile,
	})
	if err != nil {
		return false, err
	}

	svc := sts.New(sess)

	params := &sts.GetCallerIdentityInput{}

	_, err = svc.GetCallerIdentity(params)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			if awsErr.Code() == "ExpiredToken" || awsErr.Code() == "NoCredentialProviders" {
				return false, nil
			}
		}

		return false, err
	}

	//fmt.Fprintln(os.Stderr, "Running command as:", aws.StringValue(resp.Arn))
	return true, nil
}
