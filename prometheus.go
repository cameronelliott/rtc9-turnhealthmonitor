package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// cpuTemp = prometheus.NewGauge(prometheus.GaugeOpts{
	// 	Name: "cpu_temperature_celsius",
	// 	Help: "Current temperature of the CPU.",
	// })

	// hdFailures = prometheus.NewCounterVec(
	// 	prometheus.CounterOpts{
	// 		Name: "hd_errors_total",
	// 		Help: "Number of hard-disk errors.",
	// 	},
	// 	[]string{"device"},
	// )

	tot_send_msgs = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "turnmonitorx_tot_send_msgs",
			Help: "Number of messages sent",
		},
		[]string{"sourcename", "dest"})

	tot_recv_msgs = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "turnmonitorx_tot_recv_msgs",
			Help: "Number of packets received.",
		},
		[]string{"sourcename", "dest"})

	tot_lost_msgs = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "turnmonitorx_tot_lost_msgs",
			Help: "Number of packets lost.",
		},
		[]string{"sourcename", "dest"})

)

func init() {
	// Metrics have to be registered to be exposed:
	prometheus.MustRegister(tot_send_msgs)
	prometheus.MustRegister(tot_recv_msgs)
	prometheus.MustRegister(tot_lost_msgs)


}

func updatePrometheus(sourcename string, dest string, tr TurnServerTestRun) {
	//cpuTemp.Set(65.3)
	//	hdFailures.With(prometheus.Labels{"device":"/dev/sda"}).Inc()

	tot_send_msgs.With(prometheus.Labels{"sourcename": sourcename, "dest": dest}).Add(float64(tr.tot_send_msgs))

	tot_recv_msgs.With(prometheus.Labels{"sourcename": sourcename, "dest": dest}).Add(float64(tr.tot_recv_msgs))

	tot_lost_msgs.With(prometheus.Labels{"sourcename": sourcename, "dest": dest}).Add(float64(tr.tot_send_msgs - tr.tot_recv_msgs))

	//loss_percent.Collect()(prometheus.Labels{"sourcename": sourcename, "dest": dest}).Set(float64(tr.tot_send_msgs - tr.tot_recv_msgs)/float64(tr.tot_send_msgs))

}
