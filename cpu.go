package main

import (
        "flag"
        "fmt"
        "io/ioutil"
        "os"
        "os/signal"
        "path/filepath"
        "strconv"
        "strings"
        "syscall"
        "time"
)

// CPU state structure
type CPUState struct {
        Frequency   int
        Temperature float64
        Usage       float64
}

// Configuration structure
type Config struct {
        CPUType        string
        MinNormal      int
        MaxNormal      int
        MinTurbo       int
        MaxTurbo       int
        SaveGovernor   string
        TurboGovernor  string
        UsageThreshold float64
        Verbose        bool
}

// Constants
const (
        cpuFreqBasePath = "/sys/devices/system/cpu"
)

func main() {
        cpuType := flag.String("cputype", "ryzen", "CPU type (ryzen, intel, etc.)")
        minNormal := flag.Int("min-normal", 400, "Normal mode minimum frequency (MHz)")
        maxNormal := flag.Int("max-normal", 2000, "Normal mode maximum frequency (MHz)")
        minTurbo := flag.Int("min-turbo", 400, "Turbo mode minimum frequency (MHz)")
        maxTurbo := flag.Int("max-turbo", 5450, "Turbo mode maximum frequency (MHz)")
        saveGovernor := flag.String("save-governor", "powersave", "Normal mode governor")
        turboGovernor := flag.String("turbo-governor", "performance", "Turbo mode governor")
        usageThreshold := flag.Float64("cpu-usage-threshold", 60.0, "CPU usage threshold for turbo mode (%)")
        verbose := flag.Bool("verbose", false, "Enable verbose logging")

        flag.Parse()

        // root
        if os.Geteuid() != 0 {
                fmt.Println("This program must be run as root")
                os.Exit(1)
        }

        // Configuration
        config := Config{
                CPUType:        *cpuType,
                MinNormal:      *minNormal,
                MaxNormal:      *maxNormal,
                MinTurbo:       *minTurbo,
                MaxTurbo:       *maxTurbo,
                SaveGovernor:   *saveGovernor,
                TurboGovernor:  *turboGovernor,
                UsageThreshold: *usageThreshold,
                Verbose:        *verbose,
        }

        originalSettings := getCurrentSettings(0)
  
        fmt.Println("==== Configuration ====")
        fmt.Printf("CPU Type: %s\n", config.CPUType)
        fmt.Printf("Normal mode: governor %s, min freq: %d MHz, max freq: %d MHz\n",
                config.SaveGovernor, config.MinNormal, config.MaxNormal)
        fmt.Printf("Turbo mode: governor %s, min freq: %d MHz, max freq: %d MHz\n",
                config.TurboGovernor, config.MinTurbo, config.MaxTurbo)
        fmt.Printf("CPU usage threshold: %.1f%%\n", config.UsageThreshold)
        fmt.Printf("Original settings: governor %s, max freq: %d MHz\n\n",
                originalSettings.Governor, originalSettings.MaxFreq/1000)

        // ctrl+c
        setupSignalHandler(originalSettings)

        // Main
        runMonitor(config)
}

// Main monitoring loop
func runMonitor(config Config) {
        currentMode := ""
        var lastStatusTime time.Time = time.Now()
        forceApplySettings := true

        highUsageCounter := 0
        lowUsageCounter := 0
        turboValidationThreshold := 3  // Fixed 3 seconds validation
        normalValidationThreshold := 3 // Fixed 3 seconds validation

        timestamp := time.Now().Format("2006-01-02 15:04:05")
        fmt.Printf("[%s] INIT normal mode: governor=%s min=%dMHz max=%dMHz\n",
                timestamp, config.SaveGovernor, config.MinNormal, config.MaxNormal)
        fmt.Printf("[%s] INIT turbo mode: governor=%s min=%dMHz max=%dMHz\n",
                timestamp, config.TurboGovernor, config.MinTurbo, config.MaxTurbo)

        for {
                usage := getCPUUsage()
                temp := getCPUTemperature(config.CPUType)
                freq := getAverageFrequency()

                if usage >= config.UsageThreshold {
                        // High usage - increment turbo counter, reset normal counter
                        highUsageCounter++
                        lowUsageCounter = 0
                        if config.Verbose && currentMode != "turbo" {
                                fmt.Printf("[%s] HIGH usage detected: %.1f%% (%d/%d sec)\n",
                                        time.Now().Format("2006-01-02 15:04:05"), usage, highUsageCounter, turboValidationThreshold)
                        }
                } else {
                        lowUsageCounter++
                        highUsageCounter = 0

                        if config.Verbose && currentMode != "normal" {
                                fmt.Printf("[%s] LOW usage detected: %.1f%% (%d/%d sec)\n",
                                        time.Now().Format("2006-01-02 15:04:05"), usage, lowUsageCounter, normalValidationThreshold)
                        }
                }

                shouldSwitchToTurbo := currentMode != "turbo" && highUsageCounter >= turboValidationThreshold
                shouldSwitchToNormal := currentMode != "normal" && lowUsageCounter >= normalValidationThreshold
                if shouldSwitchToTurbo || shouldSwitchToNormal || forceApplySettings {
                        timestamp := time.Now().Format("2006-01-02 15:04:05")

                        if shouldSwitchToNormal || (forceApplySettings && usage < config.UsageThreshold) {
                                applySettings(config.SaveGovernor, config.MinNormal*1000, config.MaxNormal*1000)
                                fmt.Printf("[%s] SWITCH to normal mode: governor=%s min=%dMHz max=%dMHz (usage=%.1f%% for %ds)\n",
                                        timestamp, config.SaveGovernor, config.MinNormal, config.MaxNormal, usage, lowUsageCounter)
                                currentMode = "normal"
                        } else if shouldSwitchToTurbo || (forceApplySettings && usage >= config.UsageThreshold) {
                                applySettings(config.TurboGovernor, config.MinTurbo*1000, config.MaxTurbo*1000)
                                fmt.Printf("[%s] SWITCH to turbo mode: governor=%s min=%dMHz max=%dMHz (usage=%.1f%% for %ds)\n",
                                        timestamp, config.TurboGovernor, config.MinTurbo, config.MaxTurbo, usage, highUsageCounter)
                                currentMode = "turbo"
                        }

                        forceApplySettings = false
                }
                if config.Verbose || time.Since(lastStatusTime) >= 60*time.Second {
                        timestamp := time.Now().Format("2006-01-02 15:04:05")
                        fmt.Printf("[%s] STATUS: usage=%.1f%% temp=%.1f°C freq=%dMHz mode=%s\n",
                                timestamp, usage, temp, freq/1000, currentMode)
                        lastStatusTime = time.Now()
                }
                time.Sleep(1 * time.Second)
        }
}

func applySettings(governor string, minFreq int, maxFreq int) {
        cpuCount := getCPUCount()
        for i := 0; i < cpuCount; i++ {
                setMaxFrequency(i, maxFreq)
                setMinFrequency(i, minFreq)
                setGovernor(i, governor)
        }
        time.Sleep(100 * time.Millisecond)
        for i := 0; i < cpuCount; i++ {
                currMaxFreq := getMaxFrequency(i)
                currMinFreq := getMinFrequency(i)
                if currMaxFreq != maxFreq {
                        setMaxFrequency(i, maxFreq)
                }
                if currMinFreq != minFreq {
                        setMinFrequency(i, minFreq)
                }
        }
}
func getCPUUsage() float64 {
        // First reading
        data1, err := ioutil.ReadFile("/proc/stat")
        if err != nil {
                return 0
        }

        lines1 := strings.Split(string(data1), "\n")
        if len(lines1) == 0 {
                return 0
        }

        cpuLine1 := lines1[0]
        fields1 := strings.Fields(cpuLine1)
        if len(fields1) < 8 {
                return 0
        }

        user1, _ := strconv.ParseUint(fields1[1], 10, 64)
        nice1, _ := strconv.ParseUint(fields1[2], 10, 64)
        system1, _ := strconv.ParseUint(fields1[3], 10, 64)
        idle1, _ := strconv.ParseUint(fields1[4], 10, 64)
        iowait1, _ := strconv.ParseUint(fields1[5], 10, 64)

        time.Sleep(500 * time.Millisecond)

        // Second reading
        data2, err := ioutil.ReadFile("/proc/stat")
        if err != nil {
                return 0
        }

        lines2 := strings.Split(string(data2), "\n")
        cpuLine2 := lines2[0]
        fields2 := strings.Fields(cpuLine2)

        user2, _ := strconv.ParseUint(fields2[1], 10, 64)
        nice2, _ := strconv.ParseUint(fields2[2], 10, 64)
        system2, _ := strconv.ParseUint(fields2[3], 10, 64)
        idle2, _ := strconv.ParseUint(fields2[4], 10, 64)
        iowait2, _ := strconv.ParseUint(fields2[5], 10, 64)

        // Calculate deltas
        userDelta := user2 - user1
        niceDelta := nice2 - nice1
        systemDelta := system2 - system1
        idleDelta := idle2 - idle1
        iowaitDelta := iowait2 - iowait1

        active := userDelta + niceDelta + systemDelta
        idle := idleDelta + iowaitDelta
        total := active + idle

        if total == 0 {
                return 0
        }

        return 100.0 * float64(active) / float64(total)
}

func getCPUTemperature(cpuType string) float64 {
        if cpuType == "ryzen" {
                // ok
                hwmonDirs, err := ioutil.ReadDir("/sys/class/hwmon")
                if err == nil {
                        for _, dir := range hwmonDirs {
                                namePath := filepath.Join("/sys/class/hwmon", dir.Name(), "name")
                                nameData, err := ioutil.ReadFile(namePath)
                                if err == nil && strings.TrimSpace(string(nameData)) == "k10temp" {
                                        tctlPath := filepath.Join("/sys/class/hwmon", dir.Name(), "temp1_input")
                                        if data, err := ioutil.ReadFile(tctlPath); err == nil {
                                                temp, _ := strconv.Atoi(strings.TrimSpace(string(data)))
                                                return float64(temp) / 1000.0
                                        }
                                }
                        }
                }
        } else {
                // a valider
                hwmonDirs, err := ioutil.ReadDir("/sys/class/hwmon")
                if err == nil {
                        for _, dir := range hwmonDirs {
                                namePath := filepath.Join("/sys/class/hwmon", dir.Name(), "name")
                                nameData, err := ioutil.ReadFile(namePath)
                                if err == nil && strings.TrimSpace(string(nameData)) == "coretemp" {
                                        tempPath := filepath.Join("/sys/class/hwmon", dir.Name(), "temp1_input")
                                        if data, err := ioutil.ReadFile(tempPath); err == nil {
                                                temp, _ := strconv.Atoi(strings.TrimSpace(string(data)))
                                                return float64(temp) / 1000.0
                                        }
                                }
                        }
                }
        }

        thermalDirs, err := ioutil.ReadDir("/sys/class/thermal")
        if err == nil {
                for _, dir := range thermalDirs {
                        if strings.HasPrefix(dir.Name(), "thermal_zone") {
                                typePath := filepath.Join("/sys/class/thermal", dir.Name(), "type")
                                typeData, err := ioutil.ReadFile(typePath)
                                if err == nil && strings.Contains(strings.ToLower(string(typeData)), "cpu") {
                                        tempPath := filepath.Join("/sys/class/thermal", dir.Name(), "temp")
                                        if data, err := ioutil.ReadFile(tempPath); err == nil {
                                                temp, _ := strconv.Atoi(strings.TrimSpace(string(data)))
                                                return float64(temp) / 1000.0
                                        }
                                }
                        }
                }
        }
        return 0
}

func getCPUCount() int {
        files, err := ioutil.ReadDir(cpuFreqBasePath)
        if err != nil {
                return 1
        }
        count := 0
        for _, f := range files {
                if strings.HasPrefix(f.Name(), "cpu") && !strings.Contains(f.Name(), "cpuidle") && !strings.Contains(f.Name(), "cpufreq") {
                        count++
                }
        }
        return count
}

func getCurrentFrequency(cpu int) int {
        path := filepath.Join(cpuFreqBasePath, fmt.Sprintf("cpu%d/cpufreq/scaling_cur_freq", cpu))
        data, err := ioutil.ReadFile(path)
        if err != nil {
                return 0
        }
        freq, _ := strconv.Atoi(strings.TrimSpace(string(data)))
        return freq
}

func getMinFrequency(cpu int) int {
        path := filepath.Join(cpuFreqBasePath, fmt.Sprintf("cpu%d/cpufreq/scaling_min_freq", cpu))
        data, err := ioutil.ReadFile(path)
        if err != nil {
                return 0
        }
        freq, _ := strconv.Atoi(strings.TrimSpace(string(data)))
        return freq
}

func getMaxFrequency(cpu int) int {
        path := filepath.Join(cpuFreqBasePath, fmt.Sprintf("cpu%d/cpufreq/scaling_max_freq", cpu))
        data, err := ioutil.ReadFile(path)
        if err != nil {
                return 0
        }
        freq, _ := strconv.Atoi(strings.TrimSpace(string(data)))
        return freq
}

func getAverageFrequency() int {
        cpuCount := getCPUCount()
        var totalFreq int64
        var validCores int64
        for i := 0; i < cpuCount; i++ {
                freq := getCurrentFrequency(i)
                if freq > 0 {
                        totalFreq += int64(freq)
                        validCores++
                }
        }
        if validCores == 0 {
                return 0
        }
        return int(totalFreq / validCores)
}

func setMinFrequency(cpu int, freq int) {
        path := filepath.Join(cpuFreqBasePath, fmt.Sprintf("cpu%d/cpufreq/scaling_min_freq", cpu))
        ioutil.WriteFile(path, []byte(strconv.Itoa(freq)), 0644)
}

func setMaxFrequency(cpu int, freq int) {
        path := filepath.Join(cpuFreqBasePath, fmt.Sprintf("cpu%d/cpufreq/scaling_max_freq", cpu))
        ioutil.WriteFile(path, []byte(strconv.Itoa(freq)), 0644)
}

func setGovernor(cpu int, governor string) {
        path := filepath.Join(cpuFreqBasePath, fmt.Sprintf("cpu%d/cpufreq/scaling_governor", cpu))
        ioutil.WriteFile(path, []byte(governor), 0644)
}

type CPUSettings struct {
        MinFreq  int
        MaxFreq  int
        Governor string
}

func getCurrentSettings(cpu int) CPUSettings {
        minFreqPath := filepath.Join(cpuFreqBasePath, fmt.Sprintf("cpu%d/cpufreq/scaling_min_freq", cpu))
        maxFreqPath := filepath.Join(cpuFreqBasePath, fmt.Sprintf("cpu%d/cpufreq/scaling_max_freq", cpu))
        govPath := filepath.Join(cpuFreqBasePath, fmt.Sprintf("cpu%d/cpufreq/scaling_governor", cpu))

        minFreqData, _ := ioutil.ReadFile(minFreqPath)
        maxFreqData, _ := ioutil.ReadFile(maxFreqPath)
        govData, _ := ioutil.ReadFile(govPath)

        minFreq, _ := strconv.Atoi(strings.TrimSpace(string(minFreqData)))
        maxFreq, _ := strconv.Atoi(strings.TrimSpace(string(maxFreqData)))
        gov := strings.TrimSpace(string(govData))

        return CPUSettings{
                MinFreq:  minFreq,
                MaxFreq:  maxFreq,
                Governor: gov,
        }
}

// CTRL+C
func setupSignalHandler(original CPUSettings) {
        sigChan := make(chan os.Signal, 1)
        signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

        go func() {
                <-sigChan
                fmt.Println("\nRestoring original settings...")

                cpuCount := getCPUCount()
                for i := 0; i < cpuCount; i++ {
                        setGovernor(i, original.Governor)
                        setMinFrequency(i, original.MinFreq)
                        setMaxFrequency(i, original.MaxFreq)
                }

                fmt.Println("Settings restored. Goodbye!")
                os.Exit(0)
        }()
}
