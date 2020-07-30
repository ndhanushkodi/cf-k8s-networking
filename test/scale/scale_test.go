package scale_test

import (
	"math"
	"net/http"
	"time"

	"github.com/cf-k8s-networking/ci/scale/internal/collector"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"

	"fmt"

	"github.com/montanaflynn/stats"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
)

var _ = Describe("Scale", func() {
	var (
		routeMapper *collector.RouteMapper
		results     []float64
	)

	BeforeEach(func() {
		routeMapper = &collector.RouteMapper{
			Client: http.Client{
				Timeout: 1 * time.Second,
			},
		}
	})

	AfterEach(func() {
		// For development purposes, to reset the routes back to the original hostnames
		// so we can rerun the tests
		if cleanup {
			forEachAppInSpace(numApps, numAppsPerSpace, func(i int) {
				appName := fmt.Sprintf("bin-%d", i)
				routeHost := fmt.Sprintf("bin-new-%d", i)

				session := cf.Cf("delete-route", domain, "--hostname", routeHost, "-f")
				Eventually(session, "30s").Should(Exit(0))

				session = cf.Cf("map-route", appName, domain, "--hostname", appName)
				Eventually(session, "30s").Should(Exit(0))
			})

			// Print out the statistics after the test
			p95, _ := stats.Percentile(results, 95)
			min, _ := stats.Min(results)
			max, _ := stats.Max(results)
			avg, _ := stats.Mean(results)
			fmt.Fprintln(GinkgoWriter, "\n\n\n*********************************************")
			fmt.Fprintln(GinkgoWriter, "Map Route Latency Steady State Results")
			fmt.Fprintf(GinkgoWriter, "\tP95: %.2f Seconds\n", p95)
			fmt.Fprintf(GinkgoWriter, "\tMin: %.2f Seconds\n", min)
			fmt.Fprintf(GinkgoWriter, "\tMax: %.2f Seconds\n", max)
			fmt.Fprintf(GinkgoWriter, "\tAverage: %.2f Seconds\n", avg)
			fmt.Fprintln(GinkgoWriter, "*********************************************")
		}
	})

	Context("On an environment with 1000 apps and 1000 routes", func() {
		It("maps 95% of the routes within 10 seconds", func() {
			forEachAppInSpace(numApps, numAppsPerSpace, func(i int) {
				appName := fmt.Sprintf("bin-%d", i)
				routeToDelete := fmt.Sprintf("bin-%d", i)
				routeToMap := fmt.Sprintf("bin-new-%d", i)
				routeMapper.MapRoute(appName, domain, routeToDelete, routeToMap)
				time.Sleep(10 * time.Second)
			})

			routeMapper.Wait()

			results = routeMapper.GetResults()
			p95, err := stats.Percentile(results, 95)
			Expect(err).NotTo(HaveOccurred())

			Expect(p95).To(BeNumerically("<=", 10))
		})
	})
})

func forEachAppInSpace(apps, appsPerSpace int, f func(int)) {
	numOrgsSpaces := int(math.Ceil(float64(apps) / float64(appsPerSpace)))
	for n := 0; n < numOrgsSpaces; n++ {
		session := cf.Cf("target", "-o", fmt.Sprintf("%s-%d", orgNamePrefix, n), "-s", fmt.Sprintf("%s-%d", spaceNamePrefix, n))
		Eventually(session, "30s").Should(Exit(0))

		for i := 0; i < int(math.Min(float64(appsPerSpace), float64(apps))); i++ {
			appNumber := (n * appsPerSpace) + i
			f(appNumber)
		}
	}
}