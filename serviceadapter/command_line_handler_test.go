package serviceadapter_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"

	"gopkg.in/yaml.v2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/pivotal-cf/on-demand-service-broker-sdk/bosh"
	"github.com/pivotal-cf/on-demand-service-broker-sdk/serviceadapter"
	"github.com/pivotal-cf/on-demand-service-broker-sdk/serviceadapter/testharness/testvariables"
)

var _ = Describe("Command line handler", func() {
	var args []string
	var stdout *bytes.Buffer
	var stderr *bytes.Buffer

	var (
		expectedServiceDeployment = serviceadapter.ServiceDeployment{
			DeploymentName: "service-instance-deployment",
			Releases: serviceadapter.ServiceReleases{
				{
					Name:    "release-name",
					Version: "release-version",
					Jobs:    []string{"job_one", "job_two"},
				},
			},
			Stemcell: serviceadapter.Stemcell{
				OS:      "BeOS",
				Version: "2",
			},
		}

		expectedCurrentPlan = serviceadapter.Plan{
			InstanceGroups: []serviceadapter.InstanceGroup{{
				Name:           "example-server",
				VMType:         "small",
				PersistentDisk: "ten",
				Networks:       []string{"example-network"},
				AZs:            []string{"example-az"},
				Instances:      1,
				Lifecycle:      "errand",
			}},
			Properties: serviceadapter.Properties{"example": "property"},
		}
		expectedArbitraryParams = map[string]interface{}{"key": "foo", "bar": "baz"}

		expectedResultantBoshManifest = bosh.BoshManifest{Name: "deployment-name",
			Releases: []bosh.Release{
				{
					Name:    "a-release",
					Version: "latest",
				},
			},
			Stemcells: []bosh.Stemcell{
				{
					Alias:   "greatest",
					OS:      "Windows",
					Version: "3.1",
				},
			}}

		expectedPreviousPlan = serviceadapter.Plan{
			InstanceGroups: []serviceadapter.InstanceGroup{{
				Name:           "another-example-server",
				VMType:         "small",
				PersistentDisk: "ten",
				Networks:       []string{"example-network"},
				AZs:            []string{"example-az"},
				Instances:      1,
				Lifecycle:      "errand",
			}},
			Properties: serviceadapter.Properties{"example": "property"},
		}
		expectedPlan = expectedPreviousPlan

		expectedPreviousManifest = bosh.BoshManifest{Name: "another-deployment-name",
			Releases: []bosh.Release{
				{
					Name:    "a-release",
					Version: "latest",
				},
			},
			InstanceGroups: []bosh.InstanceGroup{},
			Stemcells: []bosh.Stemcell{
				{
					Alias:   "greatest",
					OS:      "Windows",
					Version: "3.1",
				},
			}}

		expectedBindingID = "bindingId"
		expectedBoshVMs   = bosh.BoshVMs{"kafka": []string{"a", "b"}}
		expectedManifest  = expectedPreviousManifest

		expectedUnbindingRequestParams = serviceadapter.RequestParameters{"unbinding_param": "unbinding_value"}
	)

	BeforeEach(func() {
		args = []string{}
		stdout = new(bytes.Buffer)
		stderr = new(bytes.Buffer)
	})

	var (
		operationFails string
		exitCode       int

		serviceDeploymentFilePath string
		planFilePath              string
		arbitraryParamsFilePath   string
		previousManifestFilePath  string
		previousPlanFilePath      string

		bindingIdFilePath     string
		boshVMsFilePath       string
		boshManifestFilePath  string
		bindingParamsFilePath string

		instanceIDFilePath        string
		dashboardPlanFilePath     string
		dashboardManifestFilePath string

		doNotImplementInterfaces bool
	)

	createTempFile := func() string {
		file, err := ioutil.TempFile("", "sdk-tests")
		Expect(err).NotTo(HaveOccurred())
		defer file.Close()
		return file.Name()
	}

	BeforeEach(func() {
		serviceDeploymentFilePath = createTempFile()
		planFilePath = createTempFile()
		arbitraryParamsFilePath = createTempFile()
		previousManifestFilePath = createTempFile()
		previousPlanFilePath = createTempFile()

		bindingIdFilePath = createTempFile()
		boshVMsFilePath = createTempFile()
		boshManifestFilePath = createTempFile()
		bindingParamsFilePath = createTempFile()

		instanceIDFilePath = createTempFile()
		dashboardPlanFilePath = createTempFile()
		dashboardManifestFilePath = createTempFile()

		doNotImplementInterfaces = false
	})

	AfterEach(func() {
		for _, filePath := range []string{
			serviceDeploymentFilePath, planFilePath, arbitraryParamsFilePath, previousManifestFilePath, previousPlanFilePath,
			bindingIdFilePath, boshVMsFilePath, boshManifestFilePath, bindingParamsFilePath,
			instanceIDFilePath, dashboardPlanFilePath, dashboardManifestFilePath,
		} {
			Expect(os.Remove(filePath)).To(Succeed())
		}
	})

	JustBeforeEach(func() {
		cmd := exec.Command(adapterBin, args...)
		cmd.Env = []string{
			fmt.Sprintf("%s=%s", testvariables.OperationFailsKey, operationFails),
			fmt.Sprintf("%s=%s", testvariables.GenerateManifestServiceDeploymentFileKey, serviceDeploymentFilePath),
			fmt.Sprintf("%s=%s", testvariables.GenerateManifestPlanFileKey, planFilePath),
			fmt.Sprintf("%s=%s", testvariables.GenerateManifestArbitraryParamsFileKey, arbitraryParamsFilePath),
			fmt.Sprintf("%s=%s", testvariables.GenerateManifestPreviousManifestFileKey, previousManifestFilePath),
			fmt.Sprintf("%s=%s", testvariables.GenerateManifestPreviousPlanFileKey, previousPlanFilePath),

			fmt.Sprintf("%s=%s", testvariables.BindingIdFileKey, bindingIdFilePath),
			fmt.Sprintf("%s=%s", testvariables.BindingVmsFileKey, boshVMsFilePath),
			fmt.Sprintf("%s=%s", testvariables.BindingManifestFileKey, boshManifestFilePath),
			fmt.Sprintf("%s=%s", testvariables.BindingParamsFileKey, bindingParamsFilePath),

			fmt.Sprintf("%s=%s", testvariables.DashboardURLInstanceIDKey, instanceIDFilePath),
			fmt.Sprintf("%s=%s", testvariables.DashboardURLPlanKey, dashboardPlanFilePath),
			fmt.Sprintf("%s=%s", testvariables.DashboardURLManifestKey, dashboardManifestFilePath),

			fmt.Sprintf("%s=%t", testvariables.DoNotImplementInterfacesKey, doNotImplementInterfaces),
		}
		runningAdapter, err := gexec.Start(cmd, io.MultiWriter(GinkgoWriter, stdout), io.MultiWriter(GinkgoWriter, stderr))
		Expect(err).NotTo(HaveOccurred())
		Eventually(runningAdapter).Should(gexec.Exit())
		exitCode = runningAdapter.ExitCode()
	})

	jsonDeserialise := func(filePath string, pointer interface{}) {
		file, err := os.Open(filePath)
		Expect(err).NotTo(HaveOccurred())
		defer file.Close()
		Expect(json.NewDecoder(file).Decode(pointer)).To(Succeed())
	}

	yamlDeserialise := func(filePath string, pointer interface{}) {
		fileBytes, err := ioutil.ReadFile(filePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(yaml.Unmarshal(fileBytes, pointer)).To(Succeed())
	}

	Context("generating a manifest", func() {
		BeforeEach(func() {
			args = []string{"generate-manifest", toJson(expectedServiceDeployment), toJson(expectedCurrentPlan), toJson(expectedArbitraryParams), "", "null"}
			operationFails = ""
		})

		var (
			actualServiceDeployment serviceadapter.ServiceDeployment
			actualCurrentPlan       serviceadapter.Plan
			actualArbitraryParams   map[string]interface{}
			actualPreviousManifest  *bosh.BoshManifest
			actualPreviousPlan      *serviceadapter.Plan
		)

		JustBeforeEach(func() {
			jsonDeserialise(serviceDeploymentFilePath, &actualServiceDeployment)
			jsonDeserialise(planFilePath, &actualCurrentPlan)
			jsonDeserialise(arbitraryParamsFilePath, &actualArbitraryParams)
			yamlDeserialise(previousManifestFilePath, &actualPreviousManifest)
			jsonDeserialise(previousPlanFilePath, &actualPreviousPlan)
		})

		It("exits with 0", func() {
			Expect(exitCode).To(Equal(0))
		})

		It("deserialises the service deployment", func() {
			Expect(actualServiceDeployment).To(Equal(expectedServiceDeployment))
		})

		It("deserialises the current plan", func() {
			Expect(actualCurrentPlan).To(Equal(expectedCurrentPlan))
		})

		It("deserialises the arbitrary params", func() {
			Expect(actualArbitraryParams).To(Equal(expectedArbitraryParams))
		})

		It("deserialises the manfiest as nil", func() {
			Expect(actualPreviousManifest).To(BeNil())
		})

		It("deserialises the previous plan as nil", func() {
			Expect(actualPreviousPlan).To(BeNil())
		})

		It("serialises the manifest as yaml", func() {
			Expect(stdout.String()).To(Equal(toYaml(expectedResultantBoshManifest)))
		})

		Context("when optional paramters are passed in", func() {
			BeforeEach(func() {
				args = []string{"generate-manifest", toJson(expectedServiceDeployment), toJson(expectedCurrentPlan), toJson(expectedArbitraryParams), toYaml(expectedPreviousManifest), toJson(expectedPreviousPlan)}
			})

			It("deserialises the manfiest from params", func() {
				Expect(actualPreviousManifest).To(Equal(&expectedPreviousManifest))
			})

			It("deserialises the previous plan from params", func() {
				Expect(actualPreviousPlan).To(Equal(&expectedPreviousPlan))
			})
		})

		Context("error generating a manifest", func() {
			BeforeEach(func() {
				operationFails = "true"
			})

			It("Fails and logs", func() {
				Expect(exitCode).To(Equal(1))
				Expect(stderr).To(ContainSubstring("not valid"))
			})
		})
	})

	Context("binding", func() {
		var (
			actualBindingId     string
			actualBoshVMs       bosh.BoshVMs
			actualBoshManifest  bosh.BoshManifest
			actualBindingParams map[string]interface{}
		)

		JustBeforeEach(func() {
			jsonDeserialise(bindingIdFilePath, &actualBindingId)
			jsonDeserialise(boshVMsFilePath, &actualBoshVMs)
			yamlDeserialise(boshManifestFilePath, &actualBoshManifest)
			jsonDeserialise(bindingParamsFilePath, &actualBindingParams)
		})

		BeforeEach(func() {
			args = []string{"create-binding", expectedBindingID, toJson(expectedBoshVMs), toYaml(expectedManifest), toJson(expectedArbitraryParams)}
			operationFails = ""
		})

		It("exits with 0", func() {
			Expect(exitCode).To(Equal(0))
		})

		It("reads the binding id", func() {
			Expect(actualBindingId).To(Equal(expectedBindingID))
		})

		It("deserializes the bosh vms", func() {
			Expect(actualBoshVMs).To(Equal(expectedBoshVMs))
		})

		It("deserializes the manifest", func() {
			Expect(actualBoshManifest).To(Equal(expectedManifest))
		})

		It("deserializes the arbitrary params", func() {
			Expect(actualBindingParams).To(Equal(expectedArbitraryParams))
		})

		It("serialises binding result as json", func() {
			Expect(stdout.String()).To(MatchJSON(toJson(testvariables.SuccessfulBinding)))
		})

		Context("binding fails", func() {
			Context("binding already exists", func() {
				BeforeEach(func() {
					operationFails = testvariables.ErrBindingAlreadyExists
				})

				It("Fails and logs", func() {
					Expect(stderr).To(ContainSubstring("[odb-sdk] creating binding"))
					Expect(exitCode).To(Equal(49))
				})
			})

			Context("app_guid isn't provided", func() {
				BeforeEach(func() {
					operationFails = testvariables.ErrAppGuidNotProvided
				})

				It("Fails and logs", func() {
					Expect(exitCode).To(Equal(42))
				})

			})

			Context("internal error", func() {
				BeforeEach(func() {
					operationFails = "true"
				})

				It("Fails and logs", func() {
					Expect(exitCode).To(Equal(1))
					Expect(stderr).To(ContainSubstring("not valid"))
				})
			})
		})
	})

	Context("unbinding", func() {
		var (
			actualBindingId     string
			actualBoshVMs       bosh.BoshVMs
			actualBoshManifest  bosh.BoshManifest
			actualRequestParams serviceadapter.RequestParameters
		)

		JustBeforeEach(func() {
			jsonDeserialise(bindingIdFilePath, &actualBindingId)
			jsonDeserialise(boshVMsFilePath, &actualBoshVMs)
			yamlDeserialise(boshManifestFilePath, &actualBoshManifest)
			jsonDeserialise(bindingParamsFilePath, &actualRequestParams)
		})

		BeforeEach(func() {
			args = []string{"delete-binding", expectedBindingID, toJson(expectedBoshVMs), toYaml(expectedManifest), toJson(expectedUnbindingRequestParams)}
			operationFails = ""
		})

		It("exits with 0", func() {
			Expect(exitCode).To(Equal(0))
		})

		It("reads the binding id", func() {
			Expect(actualBindingId).To(Equal(expectedBindingID))
		})

		It("deserializes the bosh vms", func() {
			Expect(actualBoshVMs).To(Equal(expectedBoshVMs))
		})
		It("deserializes the manifest", func() {
			Expect(actualBoshManifest).To(Equal(expectedManifest))
		})

		It("deserializes the request parameters", func() {
			Expect(actualRequestParams).To(Equal(expectedUnbindingRequestParams))
		})

		Context("unbinding fails", func() {
			BeforeEach(func() {
				operationFails = "true"
			})

			It("Fails and logs", func() {
				Expect(exitCode).To(Equal(1))
				Expect(stderr).To(ContainSubstring("not valid"))
			})
		})
	})

	Context("dashboard-url", func() {
		var (
			actualInstanceID string
			actualPlan       serviceadapter.Plan
			actualManifest   bosh.BoshManifest
		)

		JustBeforeEach(func() {
			jsonDeserialise(instanceIDFilePath, &actualInstanceID)
			jsonDeserialise(dashboardPlanFilePath, &actualPlan)
			yamlDeserialise(dashboardManifestFilePath, &actualManifest)
		})

		BeforeEach(func() {
			args = []string{"dashboard-url", "instance-identifier", toJson(expectedPlan), toYaml(expectedManifest)}
			operationFails = ""
		})

		It("exits with 0", func() {
			Expect(exitCode).To(Equal(0))
		})

		It("passes through the instance id", func() {
			Expect(actualInstanceID).To(Equal("instance-identifier"))
		})

		It("deserializes the plan", func() {
			Expect(actualPlan).To(Equal(expectedPlan))
		})

		It("deserializes the manifest", func() {
			Expect(actualManifest).To(Equal(expectedManifest))
		})

		It("returns the dashboard URL", func() {
			Expect(stdout.Bytes()).To(MatchJSON(`{ "dashboard_url": "http://dashboard.com"}`))
		})

		Context("when it fails", func() {
			BeforeEach(func() {
				operationFails = "true"
			})
			It("exits with 1", func() {
				Expect(exitCode).To(Equal(1))
			})

			It("logs the error", func() {
				Expect(stderr).To(HavePrefix("[odb-sdk]"))
				Expect(stderr).To(ContainSubstring("not valid"))
			})
		})
	})

	Describe("supporting parts of the interface", func() {
		BeforeEach(func() {
			doNotImplementInterfaces = true
		})

		Context("manifest generator isn't implemented", func() {
			BeforeEach(func() {
				args = []string{"generate-manifest", toJson(expectedServiceDeployment), toJson(expectedCurrentPlan), toJson(expectedArbitraryParams), "", "null"}
			})

			It("exits with 10", func() {
				Expect(exitCode).To(Equal(10))
			})
		})

		Context("service binder isn't implemented", func() {
			Context("create-binding", func() {
				BeforeEach(func() {
					args = []string{"create-binding", toJson(expectedServiceDeployment), toJson(expectedCurrentPlan), toJson(expectedArbitraryParams), "", "null"}
				})

				It("exits with 10", func() {
					Expect(exitCode).To(Equal(10))
				})
			})

			Context("delete-binding", func() {
				BeforeEach(func() {
					args = []string{"delete-binding", toJson(expectedServiceDeployment), toJson(expectedCurrentPlan), toJson(expectedArbitraryParams), "", "null"}
				})

				It("exits with 10", func() {
					Expect(exitCode).To(Equal(10))
				})
			})
		})

		Context("dashboard url generator isn't implemented", func() {
			BeforeEach(func() {
				args = []string{"dashboard-url", "id", toJson(expectedCurrentPlan), "null"}
			})

			It("exits with 10", func() {
				Expect(exitCode).To(Equal(10))
			})
		})
	})
})

func toYaml(obj interface{}) string {
	str, err := yaml.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return string(str)
}
func toJson(obj interface{}) string {
	str, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	return string(str)
}