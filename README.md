# CPU-Turbo-Optimizer

## Description
CPU-Turbo-Optimizer is a Go tool that dynamically controls your CPU frequency based on its actual load. It was specifically developed for AMD Ryzen processors but also works with Intel processors.

**WARNING**: This project is a Proof of Concept (POC). Use it at your own risk. The authors are not responsible for any issues that may arise from its use.

## Problem Solved
Modern processors, especially Ryzen, tend to boost to turbo mode extremely easily and frequently, even for trivial tasks (like scrolling a webpage or opening a menu). This frequency increase is often disproportionate to the actual task and significantly increases power consumption and temperature without noticeable benefit to the user.

The default CPU behavior is to "sprint" at the slightest request, when a more measured approach would often be sufficient to maintain a smooth user experience.

This script allows you to:
- Limit CPU frequency in normal mode (low utilization)
- Allow turbo mode only when really necessary
- Automatically switch between these two modes based on actual usage

**Real-world usage:** I use this tool daily on my Proxmox hypervisors and have seen an impressive decrease in power consumption. My servers went from around 120W to only 50W on average, and the best part is that I don't notice any performance difference in my daily usage. That's a substantial energy saving for machines running 24/7!

## Features
- Continuous CPU usage monitoring
- Switches between power-saving and performance modes based on a configurable threshold
- Time validation to avoid too frequent changes
- CPU temperature monitoring
- Restoration of original settings on exit
- Compatible with AMD Ryzen and Intel processors

## Prerequisites
- Linux system
- Root privileges
- Go (to compile)

## Installation
```bash
git clone https://github.com/your-username/cpu-turbo-optimizer.git
cd cpu-turbo-optimizer
go build cpu.go
```

## Usage
```bash
sudo ./cpu -cputype ryzen -min-normal 400 -max-normal 2000 -min-turbo 400 -max-turbo 5450 -cpu-usage-threshold 60
```

### Available Parameters
| Parameter | Description | Default Value |
|-----------|-------------|-------------------|
| cputype | CPU type (ryzen, intel) | ryzen |
| min-normal | Minimum frequency in normal mode (MHz) | 400 |
| max-normal | Maximum frequency in normal mode (MHz) | 2000 |
| min-turbo | Minimum frequency in turbo mode (MHz) | 400 |
| max-turbo | Maximum frequency in turbo mode (MHz) | 5450 |
| save-governor | Governor in normal mode | powersave |
| turbo-governor | Governor in turbo mode | performance |
| cpu-usage-threshold | Usage threshold for turbo mode (%) | 60.0 |
| verbose | Verbose mode | false |

## Usage Examples

### Power-saving Configuration
```bash
sudo ./cpu -max-normal 1500 -cpu-usage-threshold 75
```

### Aggressive Configuration
```bash
sudo ./cpu -max-normal 3000 -min-turbo 2000 -cpu-usage-threshold 50
```

### My configuration for Proxmox and Ryzen 7945HX
```bash
sudo ./cpu -cputype ryzen -min-normal 400 -max-normal 3500 -min-turbo 400 -max-turbo 5450 -cpu-usage-threshold 30
```
This is the configuration I use on my Proxmox systems which has reduced my consumption by more than half. I've experimented quite a bit with the parameters and this works best for me.

## Setting up as a System Service

To have CPU-Turbo-Optimizer start automatically with your system, you can install it as a systemd service.

### Example systemd service file

Create the file `/etc/systemd/system/cpu-turbo-optimizer.service`:

```ini
[Unit]
Description=CPU Turbo Optimizer Service
After=network.target

[Service]
Type=simple
ExecStart=/opt/cpu-turbo-optimizer/cpu-turbo-optimizer --cputype=ryzen --min-normal=400 --max-normal=3500 --min-turbo=400 --max-turbo=5450 --cpu-usage-threshold=30
Restart=on-failure
RestartSec=10
StandardOutput=syslog
StandardError=syslog
SyslogIdentifier=cpu-turbo-optimizer
# Security settings
User=root
CapabilityBoundingSet=CAP_SYS_NICE CAP_SYS_RESOURCE
ProtectSystem=full
ProtectHome=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
```

### Enable and start the service

```bash
# Enable the service to start automatically at boot
sudo systemctl enable cpu-turbo-optimizer.service

# Start the service immediately
sudo systemctl start cpu-turbo-optimizer.service

# Check the service status
sudo systemctl status cpu-turbo-optimizer.service
```

**Note:** In my service file, I set the CPU usage threshold to 30% instead of the default 60%. I found this offers a better balance - the CPU stays calm most of the time but reacts quickly enough when there's work to do.

## Technical Operation
1. The script monitors CPU usage every second
2. If usage exceeds the threshold for 3 consecutive seconds, it switches to turbo mode
3. If usage remains below the threshold for 3 consecutive seconds, it returns to normal mode
4. Changes are applied to all CPU cores
5. A display shows the current mode, usage, temperature, and frequency

## License
This code is completely free to use. Do what you want with it!

## Contributing
This is clearly a POC (Proof of Concept) that I coded for my needs. It works well for me, but it could certainly use improvements to adapt to other configurations.

If you have ideas, feel free to modify the code! Some suggestions:
- Better detection of different CPUs (I've mainly tested on Ryzen)
- A simple graphical interface would be nice
- Configuration via external file
- Predefined profiles (gaming, office, server...)
- Support for other operating systems

All contributions are welcome. And if it crashes on your system... well, sorry, it's a POC! 😉
