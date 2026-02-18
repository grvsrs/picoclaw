


<!DOCTYPE html>

<html class="dark" lang="en"><head>
<meta charset="utf-8"/>
<meta content="width=device-width, initial-scale=1.0" name="viewport"/>
<title>Sovereign Agent System Dashboard</title>
<script src="https://cdn.tailwindcss.com?plugins=forms,container-queries"></script>
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@300;400;500;600;700&amp;display=swap" rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&amp;display=swap" rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&amp;display=swap" rel="stylesheet"/>
<script id="tailwind-config">
        tailwind.config = {
            darkMode: "class",
            theme: {
                extend: {
                    colors: {
                        "primary": "#1754cf",
                        "background-light": "#f6f6f8",
                        "background-dark": "#0a0a0a",
                        "accent-cyan": "#00f5ff",
                        "accent-green": "#39ff14",
                    },
                    fontFamily: {
                        "display": ["Space Grotesk", "sans-serif"],
                        "mono": ["ui-monospace", "SFMono-Regular", "Menlo", "Monaco", "Consolas", "Liberation Mono", "Courier New", "monospace"]
                    },
                    borderRadius: { "DEFAULT": "0.25rem", "lg": "0.5rem", "xl": "0.75rem", "full": "9999px" },
                },
            },
        }
    </script>
<style>
        .terminal-grid {
            background-image: radial-gradient(rgba(23, 84, 207, 0.1) 1px, transparent 0);
            background-size: 24px 24px;
        }
        .glow-text {
            text-shadow: 0 0 8px rgba(57, 255, 20, 0.5);
        }
        .scanline {
            width: 100%;
            height: 100px;
            z-index: 10;
            background: linear-gradient(0deg, rgba(0, 0, 0, 0) 0%, rgba(255, 255, 255, 0.02) 50%, rgba(0, 0, 0, 0) 100%);
            opacity: 0.1;
            position: absolute;
            pointer-events: none;
        }
    </style>
</head>
<body class="bg-background-light dark:bg-background-dark font-display text-slate-900 dark:text-slate-100 min-h-screen overflow-x-hidden">
<div class="relative flex h-full min-h-screen w-full flex-col">
<!-- Top Header -->
<header class="flex items-center justify-between border-b border-white/10 bg-background-dark/80 backdrop-blur-md px-6 py-3 sticky top-0 z-50">
<div class="flex items-center gap-6">
<div class="flex items-center gap-3">
<div class="text-primary">
<span class="material-symbols-outlined text-3xl">deployed_code</span>
</div>
<h2 class="text-xl font-bold tracking-tight uppercase">Sovereign <span class="text-primary">Agent</span> System</h2>
</div>
<div class="h-6 w-px bg-white/10 hidden md:block"></div>
<div class="hidden lg:flex items-center gap-4">
<div class="flex items-center gap-2 px-3 py-1 rounded-full bg-accent-green/10 border border-accent-green/20">
<span class="size-2 rounded-full bg-accent-green animate-pulse"></span>
<span class="text-xs font-bold text-accent-green uppercase tracking-wider">Health: Operational</span>
</div>
<div class="flex items-center gap-2 px-3 py-1 rounded-full bg-primary/10 border border-primary/20">
<span class="material-symbols-outlined text-sm text-primary">shield</span>
<span class="text-xs font-bold text-primary uppercase tracking-wider">Net: Private/Tailscale</span>
</div>
</div>
</div>
<div class="flex items-center gap-4">
<div class="hidden sm:flex items-center gap-2 bg-white/5 rounded-lg px-3 py-1.5 border border-white/10">
<span class="material-symbols-outlined text-sm text-slate-400">search</span>
<input class="bg-transparent border-none focus:ring-0 text-sm w-32 uppercase placeholder:text-slate-600" placeholder="CMD_EXPLORER" type="text"/>
</div>
<div class="flex items-center gap-2">
<button class="p-2 hover:bg-white/5 rounded-lg transition-colors">
<span class="material-symbols-outlined text-slate-400">notifications</span>
</button>
<div class="size-8 rounded-lg bg-gradient-to-br from-primary to-accent-cyan flex items-center justify-center font-bold text-xs" data-alt="User profile avatar gradient">
                    SA
                </div>
</div>
</div>
</header>
<div class="flex flex-1">
<!-- Sidebar -->
<aside class="w-64 border-r border-white/10 bg-background-dark hidden md:flex flex-col p-4 gap-6">
<nav class="flex flex-col gap-1">
<a class="flex items-center gap-3 px-4 py-2.5 rounded-lg bg-primary text-white font-medium group" href="#">
<span class="material-symbols-outlined">dashboard</span>
<span>Overview</span>
</a>
<a class="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-white/5 text-slate-400 transition-all" href="#">
<span class="material-symbols-outlined">smart_toy</span>
<span>Agents</span>
</a>
<a class="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-white/5 text-slate-400 transition-all" href="#">
<span class="material-symbols-outlined">memory</span>
<span>Infrastructure</span>
</a>
<a class="flex items-center gap-3 px-4 py-2.5 rounded-lg hover:bg-white/5 text-slate-400 transition-all" href="#">
<span class="material-symbols-outlined">terminal</span>
<span>System Logs</span>
</a>
</nav>
<div class="mt-auto p-4 rounded-xl bg-white/5 border border-white/10">
<p class="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-2">Environment</p>
<div class="flex items-center justify-between mb-1">
<span class="text-xs text-slate-300">Nix Version</span>
<span class="text-xs text-accent-cyan font-mono">2.18.1</span>
</div>
<div class="flex items-center justify-between">
<span class="text-xs text-slate-300">Uptime</span>
<span class="text-xs text-slate-400 font-mono">14d 2h 45m</span>
</div>
</div>
</aside>
<!-- Main Content Area -->
<main class="flex-1 p-6 overflow-y-auto terminal-grid relative">
<div class="scanline"></div>
<!-- Dashboard Title Section -->
<div class="flex flex-col md:flex-row md:items-end justify-between mb-8 gap-4">
<div>
<h1 class="text-4xl font-black tracking-tighter uppercase mb-2">Logical <span class="text-primary italic">Architecture</span></h1>
<p class="text-slate-400 font-mono text-sm">PATH: /root/sovereign/dashboard/overview</p>
</div>
<div class="flex gap-3">
<button class="px-4 py-2 bg-white/5 border border-white/10 rounded-lg text-sm font-bold flex items-center gap-2 hover:bg-white/10 transition-all">
<span class="material-symbols-outlined text-sm">refresh</span>
                        FORCE REFRESH
                    </button>
<button class="px-4 py-2 bg-primary text-white rounded-lg text-sm font-bold flex items-center gap-2 hover:bg-primary/80 transition-all shadow-[0_0_15px_rgba(23,84,207,0.4)]">
<span class="material-symbols-outlined text-sm">add</span>
                        NEW PIPELINE
                    </button>
</div>
</div>
<!-- Flow Visualizer -->
<div class="hidden lg:flex items-center justify-around px-12 mb-4 relative z-0">
<div class="flex-1 h-px bg-gradient-to-r from-transparent via-primary/30 to-primary/50 relative">
<span class="absolute right-0 top-1/2 -translate-y-1/2 material-symbols-outlined text-primary text-xl">chevron_right</span>
</div>
<div class="flex-1 h-px bg-gradient-to-r from-primary/50 via-accent-cyan/30 to-accent-cyan/50 relative">
<span class="absolute right-0 top-1/2 -translate-y-1/2 material-symbols-outlined text-accent-cyan text-xl">chevron_right</span>
</div>
<div class="flex-1 h-px bg-gradient-to-r from-accent-cyan/50 via-accent-green/30 to-accent-green/50 relative">
<span class="absolute right-0 top-1/2 -translate-y-1/2 material-symbols-outlined text-accent-green text-xl">chevron_right</span>
</div>
</div>
<!-- The Quad-Pillar Grid -->
<div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 relative z-10">
<!-- 1. Android Command Hub -->
<div class="bg-background-dark border border-white/10 rounded-xl overflow-hidden flex flex-col group hover:border-primary/50 transition-all">
<div class="p-4 border-b border-white/10 bg-white/5 flex items-center justify-between">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-primary">smartphone</span>
<span class="text-xs font-bold uppercase tracking-widest">01. Android Hub</span>
</div>
<span class="text-[10px] font-mono text-slate-500">VOICE_ACTIVE</span>
</div>
<div class="p-4 flex-1 space-y-4">
<div class="bg-white/5 p-3 rounded-lg border border-white/5">
<div class="flex justify-between items-start mb-2">
<span class="text-[10px] font-bold text-accent-cyan uppercase">News Intelligence</span>
<span class="text-[10px] font-mono text-slate-500">2m ago</span>
</div>
<p class="text-xs text-slate-300 leading-relaxed mb-2">Summary: Market volatility detected in AI hardware sector. 12 sources verified.</p>
<div class="flex gap-1 h-4 items-end">
<div class="w-1 bg-accent-cyan h-2"></div>
<div class="w-1 bg-accent-cyan h-3"></div>
<div class="w-1 bg-accent-cyan h-1"></div>
<div class="w-1 bg-accent-cyan h-4"></div>
<div class="w-1 bg-accent-cyan h-2"></div>
<div class="w-1 bg-accent-cyan h-3"></div>
</div>
</div>
<div class="bg-white/5 p-3 rounded-lg border border-white/5">
<div class="flex justify-between items-start mb-2">
<span class="text-[10px] font-bold text-accent-cyan uppercase">Audio Report</span>
<span class="text-[10px] font-mono text-slate-500">14:20</span>
</div>
<div class="flex items-center gap-3">
<button class="size-6 rounded-full bg-primary flex items-center justify-center">
<span class="material-symbols-outlined text-xs">play_arrow</span>
</button>
<div class="flex-1 h-1 bg-white/10 rounded-full overflow-hidden">
<div class="w-1/3 h-full bg-primary"></div>
</div>
</div>
</div>
</div>
</div>
<!-- 2. Telegram Agent Layer -->
<div class="bg-background-dark border border-white/10 rounded-xl overflow-hidden flex flex-col group hover:border-accent-cyan/50 transition-all">
<div class="p-4 border-b border-white/10 bg-white/5 flex items-center justify-between">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-accent-cyan">send</span>
<span class="text-xs font-bold uppercase tracking-widest">02. Agent Layer</span>
</div>
<span class="text-[10px] font-mono text-slate-500">TG_MESH</span>
</div>
<div class="p-4 space-y-3">
<div class="flex items-center justify-between p-2.5 rounded-lg border border-accent-green/20 bg-accent-green/5">
<div class="flex items-center gap-3">
<span class="material-symbols-outlined text-accent-green text-sm">security</span>
<span class="text-xs font-bold">Sentinel</span>
</div>
<span class="text-[10px] font-mono text-accent-green uppercase tracking-tighter glow-text">HEALTHY</span>
</div>
<div class="flex flex-col gap-2 p-2.5 rounded-lg border border-primary/20 bg-primary/5">
<div class="flex items-center justify-between">
<div class="flex items-center gap-3">
<span class="material-symbols-outlined text-primary text-sm">construction</span>
<span class="text-xs font-bold">Builder</span>
</div>
<span class="text-[10px] font-mono text-primary uppercase">Deploying</span>
</div>
<div class="w-full h-1 bg-white/10 rounded-full overflow-hidden">
<div class="w-[65%] h-full bg-primary animate-pulse"></div>
</div>
</div>
<div class="flex items-center justify-between p-2.5 rounded-lg border border-accent-cyan/20 bg-accent-cyan/5">
<div class="flex items-center gap-3">
<span class="material-symbols-outlined text-accent-cyan text-sm">support_agent</span>
<span class="text-xs font-bold">Frontdesk</span>
</div>
<span class="text-[10px] font-mono text-accent-cyan uppercase">CAPTURING</span>
</div>
</div>
</div>
<!-- 3. Linux Sovereign Executor -->
<div class="bg-background-dark border border-white/10 rounded-xl overflow-hidden flex flex-col group hover:border-accent-green/50 transition-all">
<div class="p-4 border-b border-white/10 bg-white/5 flex items-center justify-between">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-accent-green">terminal</span>
<span class="text-xs font-bold uppercase tracking-widest">03. Executor</span>
</div>
<span class="text-[10px] font-mono text-slate-500">NIX_OS</span>
</div>
<div class="p-4 flex-1 font-mono">
<div class="grid grid-cols-2 gap-3 mb-4">
<div class="p-2 bg-white/5 rounded border border-white/5">
<p class="text-[10px] text-slate-500 uppercase mb-1">CPU Usage</p>
<p class="text-lg font-bold text-accent-green">24.2%</p>
</div>
<div class="p-2 bg-white/5 rounded border border-white/5">
<p class="text-[10px] text-slate-500 uppercase mb-1">RAM Usage</p>
<p class="text-lg font-bold text-accent-cyan">4.2GB</p>
</div>
</div>
<div class="text-[10px] space-y-1 text-slate-400">
<div class="flex items-center gap-2">
<span class="text-accent-green">$</span>
<span>nix-shell --run pipeline.sh</span>
</div>
<div class="flex items-center gap-2">
<span class="text-accent-cyan">#</span>
<span>Cron: task_sync @ 00:00 [OK]</span>
</div>
<div class="flex items-center gap-2">
<span class="text-accent-cyan">#</span>
<span>Cron: news_aggregator [RUNNING]</span>
</div>
<div class="mt-2 pt-2 border-t border-white/5 text-[9px] text-slate-600">
<span class="block">Kernel: 6.5.0-Sovereign-x64</span>
<span class="block">Packages: 1,402 (nix-store)</span>
</div>
</div>
</div>
</div>
<!-- 4. VS Code Orchestration -->
<div class="bg-background-dark border border-white/10 rounded-xl overflow-hidden flex flex-col group hover:border-white/40 transition-all">
<div class="p-4 border-b border-white/10 bg-white/5 flex items-center justify-between">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-slate-400">code</span>
<span class="text-xs font-bold uppercase tracking-widest">04. Orchestration</span>
</div>
<span class="text-[10px] font-mono text-slate-500">VS_CODE_OSS</span>
</div>
<div class="flex-1 flex flex-col overflow-hidden">
<div class="flex border-b border-white/5">
<div class="px-3 py-1 bg-white/10 border-r border-white/5 text-[10px] flex items-center gap-2">
<span class="material-symbols-outlined text-[10px] text-primary">data_object</span>
                                pipeline.py
                            </div>
<div class="px-3 py-1 text-[10px] text-slate-500 flex items-center gap-2">
<span class="material-symbols-outlined text-[10px]">list</span>
                                logs.txt
                            </div>
</div>
<div class="p-3 bg-black/40 flex-1 font-mono text-[10px] leading-relaxed">
<div class="text-slate-500">1 | <span class="text-accent-cyan">import</span> sovereign_core <span class="text-accent-cyan">as</span> sc</div>
<div class="text-slate-500">2 | </div>
<div class="text-slate-500">3 | <span class="text-accent-green">def</span> <span class="text-accent-cyan">init_intel_stream</span>():</div>
<div class="text-slate-500">4 |     sc.connect(<span class="text-primary">'android_hub'</span>)</div>
<div class="text-slate-500">5 |     sc.deploy_agent(<span class="text-primary">'builder'</span>)</div>
<div class="mt-4 border-t border-white/5 pt-2">
<p class="text-[9px] font-bold text-slate-600 mb-1 uppercase tracking-tighter">File Watcher Activity</p>
<p class="text-accent-green">&gt;&gt; pipeline.py modified (0.2s ago)</p>
<p class="text-slate-400">&gt;&gt; Running unit tests: 14/14 passed</p>
</div>
</div>
</div>
</div>
</div>
<!-- Detailed Activity Section -->
<div class="mt-8">
<h3 class="text-lg font-bold uppercase tracking-widest mb-4 flex items-center gap-2">
<span class="size-2 rounded-full bg-primary"></span>
                    System Pipeline Stream
                </h3>
<div class="bg-background-dark/50 border border-white/10 rounded-xl p-4 overflow-hidden">
<table class="w-full text-left font-mono text-xs">
<thead>
<tr class="text-slate-500 border-b border-white/10">
<th class="pb-3 px-2 font-medium uppercase">Timestamp</th>
<th class="pb-3 px-2 font-medium uppercase">Source</th>
<th class="pb-3 px-2 font-medium uppercase">Event</th>
<th class="pb-3 px-2 font-medium uppercase text-right">Status</th>
</tr>
</thead>
<tbody class="divide-y divide-white/5">
<tr class="hover:bg-white/5 transition-colors">
<td class="py-3 px-2 text-slate-400">14:55:21</td>
<td class="py-3 px-2 text-primary">ANDROID_VOICE</td>
<td class="py-3 px-2">Voice CMD: "Sync current intelligence to Telegram"</td>
<td class="py-3 px-2 text-right"><span class="text-accent-green">EXECUTED</span></td>
</tr>
<tr class="hover:bg-white/5 transition-colors">
<td class="py-3 px-2 text-slate-400">14:54:10</td>
<td class="py-3 px-2 text-accent-cyan">TG_FRONTDESK</td>
<td class="py-3 px-2">Captured 4 new tasks from mobile client</td>
<td class="py-3 px-2 text-right"><span class="text-accent-cyan">QUEUED</span></td>
</tr>
<tr class="hover:bg-white/5 transition-colors">
<td class="py-3 px-2 text-slate-400">14:52:05</td>
<td class="py-3 px-2 text-accent-green">LINUX_EXEC</td>
<td class="py-3 px-2">Nix derivation built for 'intel-aggregator-v2'</td>
<td class="py-3 px-2 text-right"><span class="text-accent-green">SUCCESS</span></td>
</tr>
<tr class="hover:bg-white/5 transition-colors">
<td class="py-3 px-2 text-slate-400">14:50:44</td>
<td class="py-3 px-2 text-slate-300">VS_CODE</td>
<td class="py-3 px-2">Autosave: intelligence_pipeline.py</td>
<td class="py-3 px-2 text-right"><span class="text-slate-500">SAVED</span></td>
</tr>
</tbody>
</table>
</div>
</div>
</main>
</div>
</div>
</body></html>
</
</
<!DOCTYPE html>

<html class="dark" lang="en"><head>
<meta charset="utf-8"/>
<meta content="width=device-width, initial-scale=1.0" name="viewport"/>
<title>Sovereign AI | Agent Management</title>
<script src="https://cdn.tailwindcss.com?plugins=forms,container-queries"></script>
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@300;400;500;600;700&amp;display=swap" rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&amp;display=swap" rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&amp;display=swap" rel="stylesheet"/>
<script id="tailwind-config">
        tailwind.config = {
            darkMode: "class",
            theme: {
                extend: {
                    colors: {
                        "primary": "#1754cf",
                        "background-light": "#f6f6f8",
                        "background-dark": "#0a0a0b",
                        "card-dark": "#111621",
                        "border-dark": "#1e2533",
                    },
                    fontFamily: {
                        "display": ["Space Grotesk", "sans-serif"]
                    },
                    borderRadius: {"DEFAULT": "0.25rem", "lg": "0.5rem", "xl": "0.75rem", "full": "9999px"},
                },
            },
        }
    </script>
<style>
        body {
            font-family: 'Space Grotesk', sans-serif;
        }
        .terminal-scroll::-webkit-scrollbar {
            width: 4px;
        }
        .terminal-scroll::-webkit-scrollbar-track {
            background: transparent;
        }
        .terminal-scroll::-webkit-scrollbar-thumb {
            background: #1e2533;
            border-radius: 10px;
        }
        .glow-green {
            box-shadow: 0 0 8px rgba(34, 197, 94, 0.4);
        }
        .glow-red {
            box-shadow: 0 0 8px rgba(239, 68, 68, 0.4);
        }
    </style>
</head>
<body class="bg-background-light dark:bg-background-dark text-slate-900 dark:text-slate-100 min-h-screen flex">
<!-- Sidebar Navigation -->
<aside class="w-64 border-r border-slate-200 dark:border-border-dark flex flex-col h-screen sticky top-0 bg-white dark:bg-background-dark">
<div class="p-6 flex items-center gap-3">
<div class="h-10 w-10 rounded-lg bg-primary flex items-center justify-center text-white">
<span class="material-symbols-outlined">shield_with_heart</span>
</div>
<div>
<h1 class="text-sm font-bold tracking-tight uppercase">Sovereign AI</h1>
<p class="text-[10px] text-slate-500 dark:text-slate-400 font-medium">COMMAND CENTER v2.0</p>
</div>
</div>
<nav class="flex-1 px-4 space-y-2 mt-4">
<a class="flex items-center gap-3 px-3 py-2 text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-primary/10 hover:text-primary transition-colors rounded-lg group" href="#">
<span class="material-symbols-outlined text-[20px]">dashboard</span>
<span class="text-sm font-medium">Overview</span>
</a>
<a class="flex items-center gap-3 px-3 py-2 bg-primary/10 text-primary border border-primary/20 rounded-lg group" href="#">
<span class="material-symbols-outlined text-[20px]">smart_toy</span>
<span class="text-sm font-medium">Agents</span>
</a>
<a class="flex items-center gap-3 px-3 py-2 text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-primary/10 hover:text-primary transition-colors rounded-lg group" href="#">
<span class="material-symbols-outlined text-[20px]">database</span>
<span class="text-sm font-medium">Infrastructure</span>
</a>
<a class="flex items-center gap-3 px-3 py-2 text-slate-600 dark:text-slate-400 hover:bg-slate-100 dark:hover:bg-primary/10 hover:text-primary transition-colors rounded-lg group" href="#">
<span class="material-symbols-outlined text-[20px]">terminal</span>
<span class="text-sm font-medium">Logs</span>
</a>
</nav>
<div class="p-4 mt-auto border-t border-slate-200 dark:border-border-dark">
<button class="w-full bg-primary hover:bg-primary/90 text-white font-bold py-2.5 px-4 rounded-lg flex items-center justify-center gap-2 transition-all text-sm">
<span class="material-symbols-outlined text-[18px]">add_circle</span>
                Deploy New Agent
            </button>
</div>
</aside>
<!-- Main Content Area -->
<main class="flex-1 overflow-y-auto bg-[#fafafa] dark:bg-[#0d1117] p-8">
<!-- Header -->
<div class="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-8">
<div>
<h2 class="text-3xl font-bold tracking-tight">Agent Orchestrator</h2>
<p class="text-slate-500 dark:text-slate-400">Manage and monitor your autonomous Telegram bot fleet.</p>
</div>
<div class="flex items-center gap-3">
<div class="px-4 py-2 bg-white dark:bg-card-dark border border-slate-200 dark:border-border-dark rounded-lg flex items-center gap-2">
<span class="h-2 w-2 rounded-full bg-green-500 glow-green"></span>
<span class="text-xs font-medium">System Integrity: Optimal</span>
</div>
</div>
</div>
<div class="grid grid-cols-1 xl:grid-cols-3 gap-8">
<!-- Left & Middle: Agents and Settings -->
<div class="xl:col-span-2 space-y-8">
<!-- Global Settings Panel -->
<section class="bg-white dark:bg-card-dark border border-slate-200 dark:border-border-dark rounded-xl p-6">
<h3 class="text-lg font-bold mb-4 flex items-center gap-2">
<span class="material-symbols-outlined text-primary">settings</span>
                        Global Agent Settings
                    </h3>
<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
<div class="flex items-center justify-between p-4 bg-slate-50 dark:bg-background-dark/50 border border-slate-100 dark:border-border-dark rounded-lg">
<div>
<p class="text-sm font-bold">Auto-Scale Agents</p>
<p class="text-xs text-slate-500 dark:text-slate-400">Dynamic compute allocation</p>
</div>
<label class="relative inline-flex items-center cursor-pointer">
<input checked="" class="sr-only peer" type="checkbox"/>
<div class="w-11 h-6 bg-slate-200 peer-focus:outline-none dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all dark:border-gray-600 peer-checked:bg-primary"></div>
</label>
</div>
<div class="flex items-center justify-between p-4 bg-slate-50 dark:bg-background-dark/50 border border-slate-100 dark:border-border-dark rounded-lg">
<div class="flex-1">
<p class="text-sm font-bold">Global API Key</p>
<p class="text-xs text-slate-500 dark:text-slate-400 truncate max-w-[150px]">sk-sov-•••••••••••••</p>
</div>
<button class="px-3 py-1.5 bg-primary/10 text-primary text-xs font-bold rounded-md hover:bg-primary/20 transition-colors">Rotate Key</button>
</div>
</div>
</section>
<!-- Agent Grid -->
<section>
<div class="flex items-center justify-between mb-4">
<h3 class="text-lg font-bold">Active Fleet</h3>
<div class="text-xs text-slate-500 font-medium">3 Agents Registered</div>
</div>
<div class="grid grid-cols-1 md:grid-cols-2 gap-4">
<!-- Agent Card 1: System Sentinel -->
<div class="bg-white dark:bg-card-dark border border-slate-200 dark:border-border-dark rounded-xl p-5 hover:border-primary/50 transition-all group">
<div class="flex justify-between items-start mb-4">
<div class="flex items-center gap-3">
<div class="h-12 w-12 rounded-lg bg-emerald-500/10 flex items-center justify-center text-emerald-500">
<span class="material-symbols-outlined text-[28px]">security</span>
</div>
<div>
<h4 class="font-bold">System Sentinel</h4>
<div class="flex items-center gap-1.5">
<span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>
<span class="text-[10px] uppercase font-bold text-green-500 tracking-wider">Online</span>
</div>
</div>
</div>
<button class="text-slate-400 hover:text-white"><span class="material-symbols-outlined">more_vert</span></button>
</div>
<div class="space-y-4">
<div>
<div class="flex justify-between text-[11px] mb-1 uppercase font-bold text-slate-400 tracking-tighter">
<span>Current Task</span>
<span class="text-slate-200">Monitoring Traffic</span>
</div>
<div class="h-1.5 w-full bg-slate-100 dark:bg-background-dark rounded-full overflow-hidden">
<div class="h-full bg-primary rounded-full" style="width: 85%"></div>
</div>
</div>
<div class="flex flex-wrap gap-2">
<span class="px-2 py-0.5 bg-slate-100 dark:bg-border-dark rounded text-[10px] font-bold text-slate-600 dark:text-slate-300">ROOT</span>
<span class="px-2 py-0.5 bg-slate-100 dark:bg-border-dark rounded text-[10px] font-bold text-slate-600 dark:text-slate-300">ADMIN</span>
</div>
<button class="w-full py-2 bg-primary/10 group-hover:bg-primary transition-all text-primary group-hover:text-white text-xs font-bold rounded-lg border border-primary/20">Configure</button>
</div>
</div>
<!-- Agent Card 2: Builder Agent -->
<div class="bg-white dark:bg-card-dark border border-slate-200 dark:border-border-dark rounded-xl p-5 opacity-70 grayscale hover:grayscale-0 hover:opacity-100 transition-all group">
<div class="flex justify-between items-start mb-4">
<div class="flex items-center gap-3">
<div class="h-12 w-12 rounded-lg bg-slate-500/10 flex items-center justify-center text-slate-400">
<span class="material-symbols-outlined text-[28px]">construction</span>
</div>
<div>
<h4 class="font-bold">Builder Agent</h4>
<div class="flex items-center gap-1.5">
<span class="h-1.5 w-1.5 rounded-full bg-red-500"></span>
<span class="text-[10px] uppercase font-bold text-red-500 tracking-wider">Offline</span>
</div>
</div>
</div>
<button class="text-slate-400 hover:text-white"><span class="material-symbols-outlined">more_vert</span></button>
</div>
<div class="space-y-4">
<div>
<div class="flex justify-between text-[11px] mb-1 uppercase font-bold text-slate-400 tracking-tighter">
<span>Current Task</span>
<span class="text-slate-200 italic">Idle</span>
</div>
<div class="h-1.5 w-full bg-slate-100 dark:bg-background-dark rounded-full overflow-hidden">
<div class="h-full bg-slate-600 rounded-full" style="width: 0%"></div>
</div>
</div>
<div class="flex flex-wrap gap-2">
<span class="px-2 py-0.5 bg-slate-100 dark:bg-border-dark rounded text-[10px] font-bold text-slate-600 dark:text-slate-300">FILESYSTEM</span>
<span class="px-2 py-0.5 bg-slate-100 dark:bg-border-dark rounded text-[10px] font-bold text-slate-600 dark:text-slate-300">GIT</span>
</div>
<button class="w-full py-2 bg-slate-100 dark:bg-border-dark text-slate-500 text-xs font-bold rounded-lg border border-transparent">Configure</button>
</div>
</div>
<!-- Agent Card 3: Frontdesk Agent -->
<div class="bg-white dark:bg-card-dark border border-slate-200 dark:border-border-dark rounded-xl p-5 hover:border-primary/50 transition-all group">
<div class="flex justify-between items-start mb-4">
<div class="flex items-center gap-3">
<div class="h-12 w-12 rounded-lg bg-blue-500/10 flex items-center justify-center text-blue-500">
<span class="material-symbols-outlined text-[28px]">forum</span>
</div>
<div>
<h4 class="font-bold">Frontdesk Agent</h4>
<div class="flex items-center gap-1.5">
<span class="h-1.5 w-1.5 rounded-full bg-green-500"></span>
<span class="text-[10px] uppercase font-bold text-green-500 tracking-wider">Online</span>
</div>
</div>
</div>
<button class="text-slate-400 hover:text-white"><span class="material-symbols-outlined">more_vert</span></button>
</div>
<div class="space-y-4">
<div>
<div class="flex justify-between text-[11px] mb-1 uppercase font-bold text-slate-400 tracking-tighter">
<span>Current Task</span>
<span class="text-slate-200">Routing Inquiries</span>
</div>
<div class="h-1.5 w-full bg-slate-100 dark:bg-background-dark rounded-full overflow-hidden">
<div class="h-full bg-primary rounded-full" style="width: 42%"></div>
</div>
</div>
<div class="flex flex-wrap gap-2">
<span class="px-2 py-0.5 bg-slate-100 dark:bg-border-dark rounded text-[10px] font-bold text-slate-600 dark:text-slate-300">MESSAGING</span>
<span class="px-2 py-0.5 bg-slate-100 dark:bg-border-dark rounded text-[10px] font-bold text-slate-600 dark:text-slate-300">CRM</span>
</div>
<button class="w-full py-2 bg-primary/10 group-hover:bg-primary transition-all text-primary group-hover:text-white text-xs font-bold rounded-lg border border-primary/20">Configure</button>
</div>
</div>
<!-- Add Placeholder Card -->
<div class="border-2 border-dashed border-slate-200 dark:border-border-dark rounded-xl p-5 flex flex-col items-center justify-center text-slate-400 hover:text-primary hover:border-primary cursor-pointer transition-colors group">
<span class="material-symbols-outlined text-4xl mb-2 group-hover:scale-110 transition-transform">add_circle</span>
<span class="text-sm font-bold">Deploy New Agent</span>
</div>
</div>
</section>
</div>
<!-- Right Column: System Event Stream -->
<div class="space-y-6">
<section class="bg-[#0a0c10] border border-border-dark rounded-xl flex flex-col h-[calc(100vh-160px)] sticky top-8 overflow-hidden shadow-2xl">
<div class="p-4 border-b border-border-dark bg-[#111621] flex items-center justify-between">
<div class="flex items-center gap-2">
<div class="h-2 w-2 rounded-full bg-primary animate-pulse"></div>
<h3 class="text-xs font-bold uppercase tracking-widest text-slate-300">System Event Stream</h3>
</div>
<span class="material-symbols-outlined text-slate-500 text-sm">filter_list</span>
</div>
<div class="flex-1 p-4 overflow-y-auto terminal-scroll font-mono text-[11px] space-y-3 leading-relaxed">
<div class="flex gap-3">
<span class="text-slate-600 whitespace-nowrap">[14:20:11]</span>
<p><span class="text-primary font-bold">Sentinel:</span> Monitoring node <span class="text-emerald-500">0x4a...f2</span>. No anomalies detected.</p>
</div>
<div class="flex gap-3">
<span class="text-slate-600 whitespace-nowrap">[14:20:45]</span>
<p><span class="text-blue-400 font-bold">Frontdesk:</span> Received new inquiry from user <span class="text-slate-300">@SovereignNode</span>.</p>
</div>
<div class="flex gap-3">
<span class="text-slate-600 whitespace-nowrap">[14:21:02]</span>
<p><span class="text-blue-400 font-bold">Frontdesk:</span> Inquiry routed to <span class="text-slate-300">Infrastructure Support</span>.</p>
</div>
<div class="flex gap-3 border-l border-primary/30 pl-3 py-1 bg-primary/5">
<span class="text-slate-600 whitespace-nowrap">[14:22:15]</span>
<p><span class="text-amber-500 font-bold">SYSTEM:</span> Automatic key rotation scheduled for <span class="text-slate-300">00:00 UTC</span>.</p>
</div>
<div class="flex gap-3">
<span class="text-slate-600 whitespace-nowrap">[14:23:40]</span>
<p><span class="text-primary font-bold">Sentinel:</span> Traffic spike detected on <span class="text-slate-300">Endpoint /api/v1/stream</span>.</p>
</div>
<div class="flex gap-3">
<span class="text-slate-600 whitespace-nowrap">[14:24:12]</span>
<p><span class="text-red-500 font-bold">Builder:</span> Process terminated. Waiting for manual restart.</p>
</div>
<div class="flex gap-3">
<span class="text-slate-600 whitespace-nowrap">[14:25:01]</span>
<p><span class="text-primary font-bold">Sentinel:</span> Load balancer adjusted. Latency normalized at <span class="text-emerald-500">42ms</span>.</p>
</div>
<div class="flex gap-3">
<span class="text-slate-600 whitespace-nowrap">[14:25:55]</span>
<p><span class="text-blue-400 font-bold">Frontdesk:</span> Successfully resolved session <span class="text-slate-300">#49221</span>.</p>
</div>
<!-- Filler logs for scroll effect -->
<div class="flex gap-3">
<span class="text-slate-600 whitespace-nowrap">[14:26:30]</span>
<p><span class="text-primary font-bold">Sentinel:</span> Integrity check complete. 100% compliant.</p>
</div>
<div class="flex gap-3">
<span class="text-slate-600 whitespace-nowrap">[14:27:12]</span>
<p><span class="text-primary font-bold">Sentinel:</span> Heartbeat pulse emitted.</p>
</div>
</div>
<div class="p-3 border-t border-border-dark bg-[#0a0c10] flex items-center gap-2">
<span class="text-primary font-mono text-xs font-bold">$</span>
<input class="bg-transparent border-none text-xs font-mono w-full focus:ring-0 text-slate-300 placeholder-slate-600" placeholder="Execute command..." type="text"/>
</div>
</section>
</div>
</div>
</main>
</body></html>
</
</


<!DOCTYPE html>

<html class="dark" lang="en"><head>
<meta charset="utf-8"/>
<meta content="width=device-width, initial-scale=1.0" name="viewport"/>
<title>Sovereign AI | Workflow Builder</title>
<script src="https://cdn.tailwindcss.com?plugins=forms,container-queries"></script>
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@300;400;500;600;700&amp;display=swap" rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&amp;display=swap" rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&amp;display=swap" rel="stylesheet"/>
<script id="tailwind-config">
        tailwind.config = {
            darkMode: "class",
            theme: {
                extend: {
                    colors: {
                        "primary": "#1754cf",
                        "success": "#10b981",
                        "background-light": "#f6f6f8",
                        "background-dark": "#0a0c10",
                        "surface-dark": "#111621",
                        "border-dark": "#1f2937",
                    },
                    fontFamily: {
                        "display": ["Space Grotesk", "sans-serif"],
                        "mono": ["ui-monospace", "SFMono-Regular", "Menlo", "Monaco", "Consolas", "Liberation Mono", "Courier New", "monospace"]
                    },
                    borderRadius: {
                        "DEFAULT": "0.25rem",
                        "lg": "0.5rem",
                        "xl": "0.75rem",
                        "full": "9999px"
                    },
                },
            },
        }
    </script>
<style>
        .grid-bg {
            background-image: radial-gradient(#1f2937 1px, transparent 1px);
            background-size: 24px 24px;
        }
        .node-active {
            box-shadow: 0 0 15px rgba(23, 84, 207, 0.3);
        }
        .connection-line {
            stroke-dasharray: 5;
        }
    </style>
</head>
<body class="bg-background-light dark:bg-background-dark text-slate-900 dark:text-slate-100 font-display overflow-hidden h-screen">
<div class="flex h-full w-full">
<!-- Sidebar Navigation -->
<aside class="w-64 border-r border-border-dark bg-surface-dark flex flex-col shrink-0">
<div class="p-6 flex items-center gap-3">
<div class="w-10 h-10 rounded-lg bg-primary flex items-center justify-center text-white">
<span class="material-symbols-outlined">auto_fix</span>
</div>
<div>
<h1 class="text-white text-base font-bold leading-none">Sovereign AI</h1>
<p class="text-slate-500 text-xs mt-1">v2.0.4-stable</p>
</div>
</div>
<nav class="flex-1 px-4 space-y-1">
<a class="flex items-center gap-3 px-3 py-2 text-slate-400 hover:text-white hover:bg-white/5 rounded-lg transition-colors" href="#">
<span class="material-symbols-outlined text-[22px]">dashboard</span>
<span class="text-sm font-medium">Overview</span>
</a>
<a class="flex items-center gap-3 px-3 py-2 text-slate-400 hover:text-white hover:bg-white/5 rounded-lg transition-colors" href="#">
<span class="material-symbols-outlined text-[22px]">smart_toy</span>
<span class="text-sm font-medium">Agents</span>
</a>
<a class="flex items-center gap-3 px-3 py-2 bg-primary/10 text-primary border border-primary/20 rounded-lg transition-colors" href="#">
<span class="material-symbols-outlined text-[22px]">account_tree</span>
<span class="text-sm font-medium">Infrastructure</span>
</a>
<a class="flex items-center gap-3 px-3 py-2 text-slate-400 hover:text-white hover:bg-white/5 rounded-lg transition-colors" href="#">
<span class="material-symbols-outlined text-[22px]">terminal</span>
<span class="text-sm font-medium">Logs</span>
</a>
</nav>
<div class="p-4 mt-auto">
<button class="w-full flex items-center justify-center gap-2 bg-primary hover:bg-primary/90 text-white text-sm font-bold py-2.5 rounded-lg transition-all shadow-lg shadow-primary/20">
<span class="material-symbols-outlined text-sm">add</span>
                    New Pipeline
                </button>
</div>
</aside>
<!-- Main Workspace -->
<main class="flex-1 flex flex-col min-w-0 relative">
<!-- Header/Toolbar -->
<header class="h-16 border-b border-border-dark bg-surface-dark/50 backdrop-blur-md flex items-center justify-between px-6 shrink-0 z-20">
<div class="flex items-center gap-4">
<div class="flex items-center gap-2 text-slate-300">
<span class="material-symbols-outlined text-primary">account_tree</span>
<h2 class="text-lg font-bold tracking-tight">Main Production Pipeline</h2>
</div>
<span class="px-2 py-0.5 rounded text-[10px] font-bold bg-success/20 text-success border border-success/30 uppercase tracking-wider">Active</span>
</div>
<div class="flex items-center gap-3">
<div class="flex bg-border-dark rounded-lg p-1 mr-2">
<button class="p-1.5 text-slate-400 hover:text-white hover:bg-white/5 rounded transition-all">
<span class="material-symbols-outlined text-[20px]">zoom_in</span>
</button>
<button class="p-1.5 text-slate-400 hover:text-white hover:bg-white/5 rounded transition-all">
<span class="material-symbols-outlined text-[20px]">zoom_out</span>
</button>
<div class="w-px h-4 bg-slate-700 my-auto mx-1"></div>
<button class="p-1.5 text-primary bg-primary/10 rounded transition-all">
<span class="material-symbols-outlined text-[20px]">grid_view</span>
</button>
</div>
<button class="flex items-center gap-2 bg-border-dark hover:bg-slate-700 text-white px-4 py-2 rounded-lg text-sm font-medium border border-slate-700 transition-all">
<span class="material-symbols-outlined text-[18px]">play_arrow</span>
                        Run
                    </button>
<button class="bg-primary hover:bg-primary/90 text-white px-4 py-2 rounded-lg text-sm font-bold transition-all">
                        Deploy
                    </button>
</div>
</header>
<!-- Canvas Area -->
<div class="flex-1 relative grid-bg overflow-hidden flex items-center justify-center">
<!-- SVG Connections -->
<svg class="absolute inset-0 w-full h-full pointer-events-none" xmlns="http://www.w3.org/2000/svg">
<!-- Input to Agent -->
<path class="connection-line" d="M280 400 C 350 400, 350 400, 420 400" fill="none" stroke="#1754cf" stroke-width="2"></path>
<!-- Agent to Executor -->
<path d="M620 400 C 680 400, 680 320, 740 320" fill="none" stroke="#1754cf" stroke-width="2"></path>
<!-- Executor to Orchestrator -->
<path d="M940 320 C 1000 320, 1000 400, 1060 400" fill="none" stroke="#10b981" stroke-width="2"></path>
<!-- Orchestrator to Output -->
<path d="M1260 400 C 1330 400, 1330 400, 1400 400" fill="none" stroke="#10b981" stroke-width="2"></path>
</svg>
<!-- Pipeline Nodes -->
<div class="relative w-full h-full flex items-center justify-start gap-32 px-32">
<!-- 1. Input Node -->
<div class="w-64 bg-surface-dark border border-border-dark rounded-xl p-4 node-active z-10 shrink-0">
<div class="flex items-center justify-between mb-4">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-primary text-[20px]">input</span>
<span class="text-xs font-bold text-slate-400 uppercase tracking-widest">Input</span>
</div>
<span class="w-2 h-2 rounded-full bg-primary shadow-sm shadow-primary"></span>
</div>
<div class="space-y-3">
<div class="flex items-center gap-3 p-2 bg-background-dark/50 rounded-lg border border-border-dark">
<span class="material-symbols-outlined text-slate-400 text-sm">android</span>
<span class="text-sm font-medium">Android Hook</span>
</div>
<div class="flex items-center gap-3 p-2 bg-background-dark/50 rounded-lg border border-border-dark">
<span class="material-symbols-outlined text-slate-400 text-sm">mic</span>
<span class="text-sm font-medium">Voice API</span>
</div>
</div>
</div>
<!-- 2. Agent Node -->
<div class="w-64 bg-surface-dark border border-primary/50 rounded-xl p-4 node-active z-10 shrink-0 relative">
<div class="absolute -left-3 top-1/2 -translate-y-1/2 w-6 h-6 rounded-full bg-surface-dark border border-primary flex items-center justify-center">
<div class="w-2 h-2 rounded-full bg-primary"></div>
</div>
<div class="flex items-center justify-between mb-4">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-primary text-[20px]">robot_2</span>
<span class="text-xs font-bold text-slate-400 uppercase tracking-widest">Agent</span>
</div>
</div>
<div class="p-3 bg-primary/10 rounded-lg border border-primary/20 text-center">
<h3 class="text-sm font-bold text-white mb-1">Telegram Bot Router</h3>
<p class="text-[10px] text-primary font-medium">Natural Language Understanding</p>
</div>
<div class="mt-4 text-[10px] text-slate-500 font-mono italic">
                            routing to: executor.shell
                        </div>
<div class="absolute -right-3 top-1/2 -translate-y-1/2 w-6 h-6 rounded-full bg-surface-dark border border-primary flex items-center justify-center">
<div class="w-2 h-2 rounded-full bg-primary animate-pulse"></div>
</div>
</div>
<!-- 3. Executor Node -->
<div class="w-72 bg-surface-dark border border-success/30 rounded-xl p-0 overflow-hidden node-active z-10 shrink-0">
<div class="p-4 border-b border-border-dark flex items-center justify-between">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-success text-[20px]">terminal</span>
<span class="text-xs font-bold text-slate-400 uppercase tracking-widest">Executor</span>
</div>
</div>
<div class="p-3 bg-background-dark/80 font-mono text-[11px] text-success/80">
<div class="flex gap-2">
<span class="text-slate-600">01</span>
<span>{ nix.shell }:</span>
</div>
<div class="flex gap-2">
<span class="text-slate-600">02</span>
<span class="text-white">let pkgs = import &lt;nixpkgs&gt; {};</span>
</div>
<div class="flex gap-2">
<span class="text-slate-600">03</span>
<span>in pkgs.mkShell { ... }</span>
</div>
</div>
<div class="p-3 border-t border-border-dark bg-slate-900/40">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-slate-500 text-sm">inventory_2</span>
<span class="text-xs text-slate-300">Package: media-proc-v4</span>
</div>
</div>
</div>
<!-- 4. Orchestrator Node -->
<div class="w-64 bg-surface-dark border border-border-dark rounded-xl p-4 z-10 shrink-0">
<div class="flex items-center justify-between mb-4">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-slate-400 text-[20px]">integration_instructions</span>
<span class="text-xs font-bold text-slate-400 uppercase tracking-widest">IDE Hub</span>
</div>
</div>
<div class="space-y-2">
<div class="flex items-center justify-between p-2 bg-slate-800/40 rounded-lg">
<span class="text-sm font-medium">VS Code Agent</span>
<span class="material-symbols-outlined text-success text-sm">check_circle</span>
</div>
<div class="flex items-center justify-between p-2 bg-slate-800/40 rounded-lg opacity-50">
<span class="text-sm font-medium">Git Commit</span>
<span class="material-symbols-outlined text-slate-500 text-sm">schedule</span>
</div>
</div>
</div>
<!-- 5. Output Node -->
<div class="w-60 bg-surface-dark border border-border-dark rounded-xl p-4 z-10 shrink-0">
<div class="flex items-center justify-between mb-4">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-slate-400 text-[20px]">output</span>
<span class="text-xs font-bold text-slate-400 uppercase tracking-widest">Output</span>
</div>
</div>
<div class="grid grid-cols-2 gap-2">
<div class="aspect-square bg-slate-800/50 rounded-lg flex flex-col items-center justify-center border border-dashed border-slate-700">
<span class="material-symbols-outlined text-slate-500 mb-1">description</span>
<span class="text-[10px] text-slate-400">Report</span>
</div>
<div class="aspect-square bg-slate-800/50 rounded-lg flex flex-col items-center justify-center border border-dashed border-slate-700">
<span class="material-symbols-outlined text-slate-500 mb-1">code</span>
<span class="text-[10px] text-slate-400">Code</span>
</div>
</div>
</div>
</div>
<!-- Canvas Control Floater -->
<div class="absolute bottom-6 left-1/2 -translate-x-1/2 bg-surface-dark/80 backdrop-blur-xl border border-border-dark rounded-full px-6 py-3 flex items-center gap-8 shadow-2xl z-30">
<div class="flex items-center gap-2 border-r border-border-dark pr-8">
<span class="text-[10px] font-bold text-slate-500 uppercase">Scale</span>
<span class="text-sm font-mono text-white">84%</span>
</div>
<div class="flex items-center gap-6">
<button class="text-slate-400 hover:text-white transition-colors">
<span class="material-symbols-outlined">pan_tool</span>
</button>
<button class="text-primary transition-colors">
<span class="material-symbols-outlined">near_me</span>
</button>
<button class="text-slate-400 hover:text-white transition-colors">
<span class="material-symbols-outlined">layers</span>
</button>
<button class="text-slate-400 hover:text-white transition-colors">
<span class="material-symbols-outlined">history</span>
</button>
</div>
</div>
</div>
</main>
<!-- Component Library (Right Panel) -->
<aside class="w-80 border-l border-border-dark bg-surface-dark flex flex-col shrink-0 z-30">
<div class="p-4 border-b border-border-dark">
<h3 class="text-sm font-bold text-white mb-3">Step Library</h3>
<div class="relative">
<span class="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 text-lg">search</span>
<input class="w-full bg-background-dark border-border-dark rounded-lg pl-10 pr-4 py-2 text-sm focus:ring-primary focus:border-primary" placeholder="Search components..." type="text"/>
</div>
</div>
<div class="flex-1 overflow-y-auto p-4 space-y-6">
<!-- AI Logic Category -->
<div>
<h4 class="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-3">AI &amp; Logic</h4>
<div class="space-y-2">
<div class="group p-3 bg-background-dark border border-border-dark rounded-lg hover:border-primary/50 transition-all cursor-grab active:cursor-grabbing">
<div class="flex items-center gap-3 mb-1">
<span class="material-symbols-outlined text-primary text-xl">auto_stories</span>
<span class="text-sm font-bold text-slate-200">Summarizer</span>
</div>
<p class="text-xs text-slate-500 leading-relaxed">Condense long-form data into actionable insights using LLMs.</p>
</div>
<div class="group p-3 bg-background-dark border border-border-dark rounded-lg hover:border-primary/50 transition-all cursor-grab active:cursor-grabbing">
<div class="flex items-center gap-3 mb-1">
<span class="material-symbols-outlined text-primary text-xl">psychology</span>
<span class="text-sm font-bold text-slate-200">Sentiment Engine</span>
</div>
<p class="text-xs text-slate-500 leading-relaxed">Determine emotional tone and urgency from input stream.</p>
</div>
</div>
</div>
<!-- Infrastructure Category -->
<div>
<h4 class="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-3">Infrastructure</h4>
<div class="space-y-2">
<div class="group p-3 bg-background-dark border border-border-dark rounded-lg hover:border-success/50 transition-all cursor-grab active:cursor-grabbing">
<div class="flex items-center gap-3 mb-1">
<span class="material-symbols-outlined text-success text-xl">terminal</span>
<span class="text-sm font-bold text-slate-200">Nix Script</span>
</div>
<p class="text-xs text-slate-500 leading-relaxed">Execute deterministic builds or shell tasks in Nix containers.</p>
</div>
<div class="group p-3 bg-background-dark border border-border-dark rounded-lg hover:border-success/50 transition-all cursor-grab active:cursor-grabbing">
<div class="flex items-center gap-3 mb-1">
<span class="material-symbols-outlined text-success text-xl">settings_ethernet</span>
<span class="text-sm font-bold text-slate-200">Webhook Relay</span>
</div>
<p class="text-xs text-slate-500 leading-relaxed">Forward processed data to external service endpoints.</p>
</div>
</div>
</div>
<!-- Media Category -->
<div>
<h4 class="text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-3">Processing</h4>
<div class="space-y-2">
<div class="group p-3 bg-background-dark border border-border-dark rounded-lg hover:border-orange-500/50 transition-all cursor-grab active:cursor-grabbing">
<div class="flex items-center gap-3 mb-1">
<span class="material-symbols-outlined text-orange-500 text-xl">video_settings</span>
<span class="text-sm font-bold text-slate-200">Media Processor</span>
</div>
<p class="text-xs text-slate-500 leading-relaxed">Transcode, clip, or resize media assets automatically.</p>
</div>
</div>
</div>
</div>
<div class="p-4 bg-background-dark/50 border-t border-border-dark">
<div class="flex items-center gap-3">
<div class="w-8 h-8 rounded-lg bg-slate-800 flex items-center justify-center">
<span class="material-symbols-outlined text-sm text-slate-400">help</span>
</div>
<div>
<p class="text-[11px] font-bold text-slate-200">Builder Docs</p>
<p class="text-[10px] text-slate-500">Shortcut: <kbd class="bg-slate-800 px-1 rounded">⌘ K</kbd></p>
</div>
</div>
</div>
</aside>
<!-- Properties Slide-over (Minimal version for UI representation) -->
<div class="fixed right-84 top-20 w-72 bg-surface-dark border border-border-dark rounded-xl shadow-2xl z-40 p-4 transform translate-x-[400px] pointer-events-none opacity-0">
<!-- Placeholder for a properties panel when a node is clicked -->
</div>
</div>
</body></html>
</
</
<!DOCTYPE html>

<html class="dark" lang="en"><head>
<meta charset="utf-8"/>
<meta content="width=device-width, initial-scale=1.0" name="viewport"/>
<title>Sovereign AI | Agent Log System</title>
<script src="https://cdn.tailwindcss.com?plugins=forms,container-queries"></script>
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@300;400;500;600;700&amp;family=Space+Mono&amp;display=swap" rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&amp;display=swap" rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&amp;display=swap" rel="stylesheet"/>
<script id="tailwind-config">
        tailwind.config = {
            darkMode: "class",
            theme: {
                extend: {
                    colors: {
                        "primary": "#1754cf",
                        "background-light": "#f6f6f8",
                        "background-dark": "#111621",
                        "terminal-bg": "#0a0d14",
                        "success": "#10b981",
                        "warn": "#f59e0b",
                        "error": "#ef4444",
                    },
                    fontFamily: {
                        "display": ["Space Grotesk", "sans-serif"],
                        "mono": ["Space Mono", "monospace"]
                    },
                    borderRadius: {
                        "DEFAULT": "0.25rem",
                        "lg": "0.5rem",
                        "xl": "0.75rem",
                        "full": "9999px"
                    },
                },
            },
        }
    </script>
<style>
        body {
            font-family: 'Space Grotesk', sans-serif;
        }
        .custom-scrollbar::-webkit-scrollbar {
            width: 6px;
        }
        .custom-scrollbar::-webkit-scrollbar-track {
            background: rgba(255, 255, 255, 0.05);
        }
        .custom-scrollbar::-webkit-scrollbar-thumb {
            background: #1754cf;
            border-radius: 10px;
        }
        .terminal-line:hover {
            background: rgba(23, 84, 207, 0.1);
        }
    </style>
</head>
<body class="bg-background-light dark:bg-background-dark text-slate-200 min-h-screen flex flex-col font-display">
<!-- Top Navigation Bar -->
<header class="flex items-center justify-between border-b border-primary/20 bg-background-dark/80 backdrop-blur-md px-6 py-3 sticky top-0 z-50">
<div class="flex items-center gap-8">
<div class="flex items-center gap-3">
<div class="size-8 bg-primary rounded-lg flex items-center justify-center shadow-[0_0_15px_rgba(23,84,207,0.4)]">
<span class="material-symbols-outlined text-white text-xl">shield_person</span>
</div>
<div class="flex flex-col">
<h1 class="text-white text-sm font-bold leading-tight tracking-tight uppercase">Sovereign AI</h1>
<span class="text-primary text-[10px] font-bold tracking-widest uppercase">Agent Control System</span>
</div>
</div>
<nav class="hidden md:flex items-center gap-6">
<a class="text-slate-400 hover:text-white text-sm font-medium transition-colors" href="#">Dashboard</a>
<a class="text-white border-b-2 border-primary pb-1 text-sm font-medium transition-colors" href="#">Logs</a>
<a class="text-slate-400 hover:text-white text-sm font-medium transition-colors" href="#">Agents</a>
<a class="text-slate-400 hover:text-white text-sm font-medium transition-colors" href="#">Network</a>
</nav>
</div>
<div class="flex items-center gap-4">
<div class="flex gap-2">
<button class="flex items-center justify-center p-2 rounded-lg bg-white/5 hover:bg-white/10 text-slate-400 hover:text-white transition-all">
<span class="material-symbols-outlined text-xl">terminal</span>
</button>
<button class="flex items-center justify-center p-2 rounded-lg bg-white/5 hover:bg-white/10 text-slate-400 hover:text-white transition-all">
<span class="material-symbols-outlined text-xl">notifications</span>
</button>
</div>
<div class="h-8 w-[1px] bg-white/10 mx-2"></div>
<div class="flex items-center gap-3">
<div class="text-right hidden sm:block">
<p class="text-xs font-bold text-white">SysAdmin-01</p>
<p class="text-[10px] text-slate-500 uppercase">Root Access</p>
</div>
<div class="bg-primary/20 p-0.5 rounded-full ring-2 ring-primary/30">
<div class="size-8 rounded-full bg-cover bg-center" data-alt="User profile avatar with technical abstract pattern" style="background-image: url('https://lh3.googleusercontent.com/aida-public/AB6AXuC3FbatiZgLjU8C3hYu7llzu6rzZxqkgmF4hjKx63R5DCJoWGR64-LzaCpfjkuKP0rqbGs_cRXwRgWIxzA0Uvo9KX_GTDSkY3NwMUq8BUAxdblH5bDoOubJnfF0cvsxeUbKFJgN3Rl9v-Nu93WMg1-wcRsvjm_8Jcsx_cK2baraiokB3ss20TFsKqmOno0PS2vY_SMw8GRm7kkWEf3jPf2ci-KZXBqNKRaGi_o9ZFClozqF9YK_TsPzpyVIONnGQ0A5SQ6EAt_uZBo')"></div>
</div>
</div>
</div>
</header>
<main class="flex-1 flex overflow-hidden">
<!-- Sidebar: Agent Selector -->
<aside class="w-64 border-r border-primary/10 bg-terminal-bg/50 hidden lg:flex flex-col">
<div class="p-4 border-b border-primary/10">
<p class="text-[10px] font-bold text-primary tracking-[0.2em] uppercase mb-4">Select Agent</p>
<div class="space-y-2">
<button class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg bg-primary text-white shadow-lg shadow-primary/20 group">
<span class="material-symbols-outlined text-lg">security</span>
<div class="flex flex-col items-start flex-1">
<span class="text-sm font-bold">Sentinel</span>
<span class="text-[10px] opacity-80 uppercase">Active • Securing</span>
</div>
<div class="size-2 rounded-full bg-green-400 animate-pulse"></div>
</button>
<button class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-white/5 text-slate-400 hover:text-slate-200 transition-all group">
<span class="material-symbols-outlined text-lg">construction</span>
<div class="flex flex-col items-start flex-1">
<span class="text-sm font-medium">Builder</span>
<span class="text-[10px] opacity-60 uppercase">Idle • Ready</span>
</div>
<div class="size-2 rounded-full bg-slate-600"></div>
</button>
<button class="w-full flex items-center gap-3 px-3 py-2.5 rounded-lg hover:bg-white/5 text-slate-400 hover:text-slate-200 transition-all group">
<span class="material-symbols-outlined text-lg">support_agent</span>
<div class="flex flex-col items-start flex-1">
<span class="text-sm font-medium">Frontdesk</span>
<span class="text-[10px] opacity-60 uppercase">Processing • 12%</span>
</div>
<div class="size-2 rounded-full bg-primary/60"></div>
</button>
</div>
</div>
<div class="flex-1 overflow-y-auto p-4 custom-scrollbar">
<p class="text-[10px] font-bold text-slate-500 tracking-[0.2em] uppercase mb-4">Quick Filters</p>
<div class="space-y-1">
<label class="flex items-center gap-3 px-3 py-2 cursor-pointer hover:bg-white/5 rounded-lg transition-colors group">
<input checked="" class="rounded border-primary/20 bg-background-dark text-primary focus:ring-primary size-4" type="checkbox"/>
<span class="text-sm text-slate-300 group-hover:text-white">All Events</span>
</label>
<label class="flex items-center gap-3 px-3 py-2 cursor-pointer hover:bg-white/5 rounded-lg transition-colors group">
<input class="rounded border-primary/20 bg-background-dark text-primary focus:ring-primary size-4" type="checkbox"/>
<span class="text-sm text-slate-300 group-hover:text-white">Security Audits</span>
</label>
<label class="flex items-center gap-3 px-3 py-2 cursor-pointer hover:bg-white/5 rounded-lg transition-colors group">
<input class="rounded border-primary/20 bg-background-dark text-primary focus:ring-primary size-4" type="checkbox"/>
<span class="text-sm text-slate-300 group-hover:text-white">Deployment Hooks</span>
</label>
</div>
</div>
<div class="p-4 border-t border-primary/10">
<button class="w-full py-2 flex items-center justify-center gap-2 border border-primary/30 rounded-lg text-primary text-xs font-bold hover:bg-primary hover:text-white transition-all uppercase tracking-widest">
<span class="material-symbols-outlined text-sm">add_circle</span>
                    Deploy New Agent
                </button>
</div>
</aside>
<!-- Main Content: Log Terminal -->
<section class="flex-1 flex flex-col min-w-0 bg-background-dark">
<!-- Terminal Header Controls -->
<div class="p-4 border-b border-primary/10 bg-background-dark flex flex-col md:flex-row gap-4 justify-between items-start md:items-center">
<div class="flex items-center gap-4 w-full md:w-auto">
<div class="relative w-full md:w-96 group">
<span class="material-symbols-outlined absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 text-lg group-focus-within:text-primary transition-colors">search</span>
<input class="w-full bg-white/5 border border-white/10 rounded-lg py-2 pl-10 pr-4 text-sm focus:ring-1 focus:ring-primary focus:border-primary transition-all text-white placeholder:text-slate-600" placeholder="Search logs (regex supported)..." type="text"/>
</div>
</div>
<div class="flex items-center gap-2 overflow-x-auto w-full md:w-auto pb-2 md:pb-0">
<div class="flex h-8 shrink-0 items-center justify-center gap-x-2 rounded-lg bg-primary/20 border border-primary/40 px-3 cursor-pointer">
<div class="size-2 rounded-full bg-primary animate-pulse"></div>
<p class="text-primary text-[10px] font-bold uppercase tracking-wider">Info</p>
</div>
<div class="flex h-8 shrink-0 items-center justify-center gap-x-2 rounded-lg bg-success/10 border border-success/30 px-3 cursor-pointer hover:bg-success/20 transition-colors">
<div class="size-2 rounded-full bg-success"></div>
<p class="text-success text-[10px] font-bold uppercase tracking-wider">Success</p>
</div>
<div class="flex h-8 shrink-0 items-center justify-center gap-x-2 rounded-lg bg-warn/10 border border-warn/30 px-3 cursor-pointer hover:bg-warn/20 transition-colors">
<div class="size-2 rounded-full bg-warn"></div>
<p class="text-warn text-[10px] font-bold uppercase tracking-wider">Warn</p>
</div>
<div class="flex h-8 shrink-0 items-center justify-center gap-x-2 rounded-lg bg-error/10 border border-error/30 px-3 cursor-pointer hover:bg-error/20 transition-colors">
<div class="size-2 rounded-full bg-error"></div>
<p class="text-error text-[10px] font-bold uppercase tracking-wider">Error</p>
</div>
</div>
</div>
<!-- The Terminal -->
<div class="flex-1 relative overflow-hidden bg-terminal-bg flex flex-col m-4 rounded-xl border border-primary/20 shadow-2xl">
<!-- Terminal Toolbar -->
<div class="bg-white/5 border-b border-white/10 px-4 py-2 flex items-center justify-between">
<div class="flex items-center gap-2">
<div class="flex gap-1.5 mr-4">
<div class="size-3 rounded-full bg-error/40"></div>
<div class="size-3 rounded-full bg-warn/40"></div>
<div class="size-3 rounded-full bg-success/40"></div>
</div>
<span class="text-[10px] text-slate-500 font-mono tracking-tight uppercase">Terminal: sovereign-agent-sentinel.log</span>
</div>
<div class="flex items-center gap-4">
<button class="flex items-center gap-1.5 text-slate-400 hover:text-white transition-colors">
<span class="material-symbols-outlined text-sm">pause_circle</span>
<span class="text-[10px] font-bold uppercase tracking-widest">Pause Feed</span>
</button>
<button class="flex items-center gap-1.5 text-primary hover:text-primary/80 transition-colors">
<span class="material-symbols-outlined text-sm">download</span>
<span class="text-[10px] font-bold uppercase tracking-widest">Export</span>
</button>
</div>
</div>
<!-- Log Stream -->
<div class="flex-1 overflow-y-auto font-mono text-[13px] leading-6 p-4 custom-scrollbar bg-[linear-gradient(rgba(10,13,20,0.8),rgba(10,13,20,0.8)),url('https://www.transparenttextures.com/patterns/carbon-fibre.png')]">
<div class="flex gap-4 terminal-line transition-colors">
<span class="text-slate-700 select-none w-10 text-right">001</span>
<span class="text-slate-500 shrink-0">14:20:01.034</span>
<span class="text-primary font-bold uppercase w-16">[INFO]</span>
<span class="text-slate-300">System initialization sequence started. Loading core modules...</span>
</div>
<div class="flex gap-4 terminal-line transition-colors">
<span class="text-slate-700 select-none w-10 text-right">002</span>
<span class="text-slate-500 shrink-0">14:20:01.156</span>
<span class="text-primary font-bold uppercase w-16">[INFO]</span>
<span class="text-slate-300">Kernel version: <span class="text-primary">6.5.0-Sovereign-AI</span>. Platform: nixos-x86_64</span>
</div>
<div class="flex gap-4 terminal-line transition-colors">
<span class="text-slate-700 select-none w-10 text-right">003</span>
<span class="text-slate-500 shrink-0">14:20:02.481</span>
<span class="text-success font-bold uppercase w-16">[SUCCESS]</span>
<span class="text-slate-300">Nix flake derivation instantiated successfully. Environment locked.</span>
</div>
<div class="flex gap-4 terminal-line transition-colors">
<span class="text-slate-700 select-none w-10 text-right">004</span>
<span class="text-slate-500 shrink-0">14:20:03.001</span>
<span class="text-primary font-bold uppercase w-16">[INFO]</span>
<span class="text-slate-300">Checking network connectivity... Establishing Tailscale tunnel.</span>
</div>
<div class="flex gap-4 terminal-line transition-colors">
<span class="text-slate-700 select-none w-10 text-right">005</span>
<span class="text-slate-500 shrink-0">14:20:03.922</span>
<span class="text-warn font-bold uppercase w-16">[WARN]</span>
<span class="text-slate-300 italic">Latency detected in node 'nyc-edge-04'. RTT: 124ms. Failover enabled.</span>
</div>
<div class="flex gap-4 terminal-line transition-colors bg-error/10">
<span class="text-slate-700 select-none w-10 text-right">006</span>
<span class="text-slate-500 shrink-0">14:20:04.110</span>
<span class="text-error font-bold uppercase w-16">[ERROR]</span>
<span class="text-slate-200 font-bold">Failed to sync agent metadata: UNAUTHORIZED. Retrying with secondary key...</span>
</div>
<div class="flex gap-4 terminal-line transition-colors">
<span class="text-slate-700 select-none w-10 text-right">007</span>
<span class="text-slate-500 shrink-0">14:20:05.503</span>
<span class="text-success font-bold uppercase w-16">[SUCCESS]</span>
<span class="text-slate-300">Authentication successful. Sentinel agent operational status: <span class="text-success">NOMINAL</span></span>
</div>
<div class="flex gap-4 terminal-line transition-colors">
<span class="text-slate-700 select-none w-10 text-right">008</span>
<span class="text-slate-500 shrink-0">14:20:05.892</span>
<span class="text-primary font-bold uppercase w-16">[INFO]</span>
<span class="text-slate-300">Monitoring incoming telemetry for <span class="text-primary">production-cluster-A</span>...</span>
</div>
<div class="flex gap-4 terminal-line transition-colors">
<span class="text-slate-700 select-none w-10 text-right">009</span>
<span class="text-slate-500 shrink-0">14:21:12.001</span>
<span class="text-primary font-bold uppercase w-16">[INFO]</span>
<span class="text-slate-300">Agent 'Builder' requested resource lock: CPU-4-8. Status: GRANTED.</span>
</div>
<div class="flex gap-4 terminal-line transition-colors">
<span class="text-slate-700 select-none w-10 text-right">010</span>
<span class="text-slate-500 shrink-0">14:21:14.332</span>
<span class="text-success font-bold uppercase w-16">[SUCCESS]</span>
<span class="text-slate-300">Incremental build finished in 2.3s. No regressions found.</span>
</div>
<!-- Current Command Cursor -->
<div class="flex gap-4 mt-2">
<span class="text-slate-700 select-none w-10 text-right">011</span>
<span class="text-primary shrink-0">user@sovereign:~$</span>
<span class="text-white border-r-2 border-primary animate-pulse w-2"></span>
</div>
</div>
<!-- Input Bar -->
<div class="p-3 bg-white/5 border-t border-white/10 flex items-center gap-3">
<span class="material-symbols-outlined text-primary text-xl">keyboard_double_arrow_right</span>
<input class="flex-1 bg-transparent border-none text-white focus:ring-0 placeholder:text-slate-700 font-mono text-sm p-0" placeholder="Enter command or override..." type="text"/>
<div class="flex items-center gap-3">
<span class="text-[10px] text-slate-500 font-mono">SSH: ACTIVE</span>
<div class="h-4 w-[1px] bg-white/10"></div>
<span class="text-[10px] text-slate-500 font-mono uppercase tracking-widest">UTF-8</span>
</div>
</div>
</div>
</section>
<!-- Right Panel: Environment & Health -->
<aside class="w-80 border-l border-primary/10 bg-terminal-bg/50 hidden xl:flex flex-col p-4 overflow-y-auto custom-scrollbar">
<!-- Health Section -->
<div class="mb-8">
<div class="flex items-center justify-between mb-4">
<p class="text-[10px] font-bold text-primary tracking-[0.2em] uppercase">Active Processes</p>
<span class="material-symbols-outlined text-slate-500 text-lg hover:text-white cursor-pointer transition-colors">refresh</span>
</div>
<div class="space-y-4">
<div class="bg-white/5 rounded-lg p-3 border border-white/5">
<div class="flex justify-between items-center mb-2">
<span class="text-xs font-bold text-slate-200">Sentinel Main Node</span>
<span class="text-[10px] text-primary">PID: 4122</span>
</div>
<div class="space-y-3">
<div>
<div class="flex justify-between text-[10px] text-slate-400 mb-1 uppercase tracking-tighter">
<span>CPU Usage</span>
<span>14.2%</span>
</div>
<div class="h-1 w-full bg-white/10 rounded-full overflow-hidden">
<div class="h-full bg-primary rounded-full" style="width: 14.2%"></div>
</div>
</div>
<div>
<div class="flex justify-between text-[10px] text-slate-400 mb-1 uppercase tracking-tighter">
<span>Memory</span>
<span>2.4 GB / 8 GB</span>
</div>
<div class="h-1 w-full bg-white/10 rounded-full overflow-hidden">
<div class="h-full bg-primary rounded-full" style="width: 30%"></div>
</div>
</div>
</div>
</div>
<div class="bg-white/5 rounded-lg p-3 border border-white/5">
<div class="flex justify-between items-center mb-2">
<span class="text-xs font-bold text-slate-200">Builder Daemon</span>
<span class="text-[10px] text-primary">PID: 8891</span>
</div>
<div class="space-y-3">
<div>
<div class="flex justify-between text-[10px] text-slate-400 mb-1 uppercase tracking-tighter">
<span>CPU Usage</span>
<span>2.1%</span>
</div>
<div class="h-1 w-full bg-white/10 rounded-full overflow-hidden">
<div class="h-full bg-slate-500 rounded-full" style="width: 2.1%"></div>
</div>
</div>
<div>
<div class="flex justify-between text-[10px] text-slate-400 mb-1 uppercase tracking-tighter">
<span>Memory</span>
<span>128 MB</span>
</div>
<div class="h-1 w-full bg-white/10 rounded-full overflow-hidden">
<div class="h-full bg-slate-500 rounded-full" style="width: 5%"></div>
</div>
</div>
</div>
</div>
</div>
</div>
<!-- Environment Details -->
<div>
<p class="text-[10px] font-bold text-primary tracking-[0.2em] uppercase mb-4">Environment Status</p>
<div class="space-y-2">
<!-- Status Card -->
<div class="bg-white/5 rounded-lg border border-white/5 p-3 flex items-center justify-between group hover:bg-white/10 transition-all cursor-default">
<div class="flex items-center gap-3">
<div class="size-8 rounded bg-primary/20 flex items-center justify-center">
<span class="material-symbols-outlined text-primary text-lg">box</span>
</div>
<div>
<p class="text-xs font-bold text-white">Nix Shell</p>
<p class="text-[10px] text-success font-medium">Stable • Flake Lock</p>
</div>
</div>
<span class="material-symbols-outlined text-slate-500 text-sm group-hover:text-white">settings</span>
</div>
<div class="bg-white/5 rounded-lg border border-white/5 p-3 flex items-center justify-between group hover:bg-white/10 transition-all cursor-default">
<div class="flex items-center gap-3">
<div class="size-8 rounded bg-success/20 flex items-center justify-center">
<span class="material-symbols-outlined text-success text-lg">vpn_lock</span>
</div>
<div>
<p class="text-xs font-bold text-white">Tailscale</p>
<p class="text-[10px] text-success font-medium">Connected • 10.0.42.1</p>
</div>
</div>
<span class="material-symbols-outlined text-slate-500 text-sm group-hover:text-white">sync</span>
</div>
<div class="bg-white/5 rounded-lg border border-white/5 p-3 flex items-center justify-between group hover:bg-white/10 transition-all cursor-default opacity-50">
<div class="flex items-center gap-3">
<div class="size-8 rounded bg-slate-500/20 flex items-center justify-center">
<span class="material-symbols-outlined text-slate-400 text-lg">database</span>
</div>
<div>
<p class="text-xs font-bold text-white">PostgreSQL</p>
<p class="text-[10px] text-slate-500 font-medium">Disabled • Standby</p>
</div>
</div>
<span class="material-symbols-outlined text-slate-500 text-sm group-hover:text-white">play_arrow</span>
</div>
</div>
</div>
<!-- Bottom Stats -->
<div class="mt-auto pt-6">
<div class="bg-primary/10 rounded-xl p-4 border border-primary/20">
<p class="text-[10px] font-bold text-primary uppercase tracking-widest mb-2">System Uptime</p>
<p class="text-2xl font-bold text-white font-mono tracking-tighter">12d 04h 12m</p>
<div class="flex items-center justify-between mt-4">
<div class="text-center">
<p class="text-[9px] text-slate-500 uppercase tracking-tighter">Errors (24h)</p>
<p class="text-sm font-bold text-error">12</p>
</div>
<div class="text-center">
<p class="text-[9px] text-slate-500 uppercase tracking-tighter">Success Rate</p>
<p class="text-sm font-bold text-success">99.8%</p>
</div>
<div class="text-center">
<p class="text-[9px] text-slate-500 uppercase tracking-tighter">Active Tasks</p>
<p class="text-sm font-bold text-primary">07</p>
</div>
</div>
</div>
</div>
</aside>
</main>
<!-- Footer / Breadcrumb Bar -->
<footer class="h-8 bg-background-dark border-t border-primary/10 flex items-center px-4 justify-between z-50">
<div class="flex items-center gap-2 text-[10px] font-bold uppercase tracking-widest">
<span class="text-slate-500">Sovereign AI</span>
<span class="text-primary">/</span>
<span class="text-slate-500">System</span>
<span class="text-primary">/</span>
<span class="text-white">Sentinel Logs</span>
</div>
<div class="flex items-center gap-4">
<div class="flex items-center gap-2">
<div class="size-2 rounded-full bg-success"></div>
<span class="text-[10px] text-slate-400 font-bold uppercase tracking-widest">All Systems Operational</span>
</div>
<div class="h-4 w-[1px] bg-white/10"></div>
<span class="text-[10px] text-slate-500 font-mono">v4.2.1-stable</span>
</div>
</footer>
</body></html>

<!DOCTYPE html>

<html class="dark" lang="en"><head>
<meta charset="utf-8"/>
<meta content="width=device-width, initial-scale=1.0" name="viewport"/>
<title>Sovereign Agent System - Project Manifest</title>
<script src="https://cdn.tailwindcss.com?plugins=forms,container-queries"></script>
<link href="https://fonts.googleapis.com/css2?family=Space+Grotesk:wght@300;400;500;600;700&amp;display=swap" rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&amp;display=swap" rel="stylesheet"/>
<link href="https://fonts.googleapis.com/css2?family=Material+Symbols+Outlined:wght,FILL@100..700,0..1&amp;display=swap" rel="stylesheet"/>
<script id="tailwind-config">
        tailwind.config = {
            darkMode: "class",
            theme: {
                extend: {
                    colors: {
                        "primary": "#1754cf",
                        "background-light": "#f6f6f8",
                        "background-dark": "#111621",
                        "terminal-green": "#0bda5e",
                        "warning": "#fa6238",
                    },
                    fontFamily: {
                        "display": ["Space Grotesk"]
                    },
                    borderRadius: {"DEFAULT": "0.25rem", "lg": "0.5rem", "xl": "0.75rem", "full": "9999px"},
                },
            },
        }
    </script>
<style>
        .glow-primary { box-shadow: 0 0 15px rgba(23, 84, 207, 0.3); }
        .glow-success { box-shadow: 0 0 10px rgba(11, 218, 94, 0.2); }
        .terminal-bg { background: linear-gradient(180deg, rgba(17, 22, 33, 1) 0%, rgba(13, 18, 28, 1) 100%); }
    </style>
</head>
<body class="bg-background-light dark:bg-background-dark font-display text-slate-200 antialiased selection:bg-primary/30">
<div class="relative flex h-auto min-h-screen w-full flex-col overflow-x-hidden">
<!-- Top Navigation Bar -->
<header class="flex items-center justify-between whitespace-nowrap border-b border-solid border-white/10 bg-background-dark/80 backdrop-blur-md px-6 py-3 sticky top-0 z-50">
<div class="flex items-center gap-8">
<div class="flex items-center gap-3 text-primary">
<span class="material-symbols-outlined text-3xl">deployed_code</span>
<h2 class="text-white text-lg font-bold leading-tight tracking-tight">SOVEREIGN AGENT SYSTEM</h2>
</div>
<nav class="hidden md:flex items-center gap-6">
<a class="text-white text-sm font-medium hover:text-primary transition-colors border-b-2 border-primary pb-1" href="#">Manifest</a>
<a class="text-slate-400 text-sm font-medium hover:text-white transition-colors" href="#">Overview</a>
<a class="text-slate-400 text-sm font-medium hover:text-white transition-colors" href="#">Agents</a>
<a class="text-slate-400 text-sm font-medium hover:text-white transition-colors" href="#">Workflows</a>
<a class="text-slate-400 text-sm font-medium hover:text-white transition-colors" href="#">Terminal</a>
</nav>
</div>
<div class="flex items-center gap-4">
<div class="hidden sm:flex items-center bg-white/5 rounded-lg px-3 py-1.5 border border-white/10">
<span class="material-symbols-outlined text-sm text-slate-400 mr-2">search</span>
<input class="bg-transparent border-none focus:ring-0 text-sm w-40 placeholder:text-slate-500" placeholder="Command (Ctrl+K)" type="text"/>
</div>
<div class="flex gap-2">
<button class="p-2 rounded-lg bg-white/5 border border-white/10 hover:bg-white/10 transition-colors text-slate-300">
<span class="material-symbols-outlined text-xl">notifications</span>
</button>
<button class="p-2 rounded-lg bg-white/5 border border-white/10 hover:bg-white/10 transition-colors text-slate-300">
<span class="material-symbols-outlined text-xl">settings</span>
</button>
</div>
<div class="size-9 rounded-full bg-primary flex items-center justify-center font-bold text-white border border-white/20">
                JD
            </div>
</div>
</header>
<main class="flex-1 max-w-[1440px] mx-auto w-full p-6 space-y-6">
<!-- Dashboard Header / Mission Briefing -->
<div class="flex flex-col lg:flex-row lg:items-end justify-between gap-6 bg-primary/5 border border-primary/20 rounded-xl p-8 relative overflow-hidden">
<div class="absolute top-0 right-0 p-4 opacity-10">
<span class="material-symbols-outlined text-9xl">shield_person</span>
</div>
<div class="relative z-10">
<div class="flex items-center gap-2 mb-2">
<span class="h-2 w-2 rounded-full bg-terminal-green animate-pulse"></span>
<span class="text-xs font-bold tracking-widest text-primary uppercase">System Operational</span>
</div>
<h1 class="text-white text-4xl font-black tracking-tight mb-2">PROJECT MANIFEST</h1>
<p class="text-slate-400 text-sm max-w-xl">Environment: <span class="text-slate-200">PROD_SWARM_ALPHA</span> | Runtime: <span class="text-slate-200">v4.8.2-stable</span> | Node: <span class="text-slate-200">SA_DIST_01</span></p>
</div>
<div class="flex flex-wrap gap-3 relative z-10">
<button class="flex items-center gap-2 px-5 py-2.5 bg-primary text-white font-bold rounded-lg hover:bg-primary/90 transition-all glow-primary">
<span class="material-symbols-outlined text-xl">bolt</span>
                    Deploy New Agent
                </button>
<button class="flex items-center gap-2 px-5 py-2.5 bg-white/5 text-white font-bold rounded-lg border border-white/10 hover:bg-white/10 transition-all">
<span class="material-symbols-outlined text-xl">ios_share</span>
                    Export Manifest
                </button>
</div>
</div>
<!-- System Integration Matrix -->
<div class="grid grid-cols-2 md:grid-cols-4 gap-4">
<div class="bg-white/5 border border-white/10 rounded-xl p-5 flex flex-col gap-1">
<span class="text-slate-500 text-xs font-bold uppercase tracking-wider">Swarm Uptime</span>
<div class="flex items-baseline gap-2">
<span class="text-2xl font-bold text-white tracking-tighter">99.982%</span>
<span class="text-terminal-green text-xs font-medium">+0.002</span>
</div>
</div>
<div class="bg-white/5 border border-white/10 rounded-xl p-5 flex flex-col gap-1">
<span class="text-slate-500 text-xs font-bold uppercase tracking-wider">Active Agents</span>
<div class="flex items-baseline gap-2">
<span class="text-2xl font-bold text-white tracking-tighter">14/14</span>
<span class="text-primary text-xs font-medium">Full Capacity</span>
</div>
</div>
<div class="bg-white/5 border border-white/10 rounded-xl p-5 flex flex-col gap-1">
<span class="text-slate-500 text-xs font-bold uppercase tracking-wider">Workflows/Hr</span>
<div class="flex items-baseline gap-2">
<span class="text-2xl font-bold text-white tracking-tighter">1,204</span>
<span class="text-warning text-xs font-medium">-4.2%</span>
</div>
</div>
<div class="bg-white/5 border border-white/10 rounded-xl p-5 flex flex-col gap-1">
<span class="text-slate-500 text-xs font-bold uppercase tracking-wider">Avg Swarm Latency</span>
<div class="flex items-baseline gap-2">
<span class="text-2xl font-bold text-white tracking-tighter">42ms</span>
<span class="text-terminal-green text-xs font-medium">Optimal</span>
</div>
</div>
</div>
<!-- Bento Grid Modules -->
<div class="grid grid-cols-1 md:grid-cols-12 gap-6 h-auto">
<!-- Module 1: System Overview (Architecture Mini-Map) -->
<div class="md:col-span-8 bg-white/5 border border-white/10 rounded-xl overflow-hidden group hover:border-primary/50 transition-all">
<div class="p-4 border-b border-white/10 flex justify-between items-center bg-white/5">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-primary">hub</span>
<h3 class="font-bold text-white">System Overview</h3>
<span class="text-[10px] px-2 py-0.5 rounded-full bg-primary/20 text-primary border border-primary/30 uppercase tracking-widest font-black">Architecture</span>
</div>
<button class="text-slate-400 hover:text-white"><span class="material-symbols-outlined text-xl">open_in_full</span></button>
</div>
<div class="p-6">
<div class="relative w-full h-64 bg-background-dark rounded-lg border border-white/5 overflow-hidden flex items-center justify-center">
<!-- Simulated Architecture Map -->
<div class="absolute inset-0 opacity-20 pointer-events-none" style="background-image: radial-gradient(circle, #1754cf 1px, transparent 1px); background-size: 30px 30px;"></div>
<div class="relative z-10 flex flex-col items-center gap-8">
<div class="px-4 py-2 bg-primary rounded border border-white/20 text-xs font-bold text-white">CENTRAL_CORE</div>
<div class="flex gap-12">
<div class="flex flex-col items-center gap-2">
<div class="size-10 rounded-lg bg-slate-800 border border-white/10 flex items-center justify-center"><span class="material-symbols-outlined text-primary">security</span></div>
<span class="text-[10px] text-slate-400 font-mono">SENTINEL_V1</span>
</div>
<div class="flex flex-col items-center gap-2">
<div class="size-10 rounded-lg bg-slate-800 border border-white/10 flex items-center justify-center"><span class="material-symbols-outlined text-primary">build</span></div>
<span class="text-[10px] text-slate-400 font-mono">BUILDER_V4</span>
</div>
<div class="flex flex-col items-center gap-2">
<div class="size-10 rounded-lg bg-slate-800 border border-white/10 flex items-center justify-center"><span class="material-symbols-outlined text-primary">support_agent</span></div>
<span class="text-[10px] text-slate-400 font-mono">FRONTDESK_V2</span>
</div>
</div>
</div>
<!-- Connections Overlay SVG -->
<svg class="absolute inset-0 w-full h-full opacity-30">
<line stroke="#1754cf" stroke-width="2" x1="50%" x2="30%" y1="35%" y2="55%"></line>
<line stroke="#1754cf" stroke-width="2" x1="50%" x2="50%" y1="35%" y2="55%"></line>
<line stroke="#1754cf" stroke-width="2" x1="50%" x2="70%" y1="35%" y2="55%"></line>
</svg>
</div>
</div>
</div>
<!-- Telemetry Sidebar -->
<div class="md:col-span-4 space-y-6">
<div class="bg-white/5 border border-white/10 rounded-xl p-5">
<h3 class="text-sm font-bold text-white mb-4 flex items-center gap-2 uppercase tracking-wider">
<span class="material-symbols-outlined text-slate-400 text-lg">monitoring</span> Swarm Telemetry
                    </h3>
<div class="space-y-4">
<div>
<div class="flex justify-between text-xs mb-1.5">
<span class="text-slate-400">Total CPU Load</span>
<span class="text-white font-mono">24.2%</span>
</div>
<div class="h-1.5 w-full bg-white/5 rounded-full overflow-hidden border border-white/5">
<div class="h-full bg-primary" style="width: 24.2%"></div>
</div>
</div>
<div>
<div class="flex justify-between text-xs mb-1.5">
<span class="text-slate-400">Memory Allocation</span>
<span class="text-white font-mono">8.4GB / 32GB</span>
</div>
<div class="h-1.5 w-full bg-white/5 rounded-full overflow-hidden border border-white/5">
<div class="h-full bg-primary" style="width: 28%"></div>
</div>
</div>
<div class="pt-4 border-t border-white/10">
<p class="text-[10px] text-slate-500 font-mono leading-relaxed">
                                &gt; AGENT_SWARM_HEARTBEAT: OK<br/>
                                &gt; PACKET_LOSS: 0.0001%<br/>
                                &gt; NODES_ACTIVE: 128
                            </p>
</div>
</div>
</div>
<div class="bg-primary/10 border border-primary/30 rounded-xl p-5 relative overflow-hidden">
<div class="absolute -right-4 -bottom-4 opacity-10">
<span class="material-symbols-outlined text-7xl">security_update_good</span>
</div>
<h3 class="text-sm font-bold text-primary mb-2 flex items-center gap-2 uppercase tracking-wider">
                        Security Integrity
                    </h3>
<p class="text-xs text-slate-300 leading-normal">
                        All agents are operating within designated sovereign boundaries. No unauthorized escalations detected.
                    </p>
<div class="mt-4 flex items-center gap-2">
<span class="px-2 py-0.5 bg-terminal-green/20 text-terminal-green border border-terminal-green/30 text-[10px] font-bold rounded">ENCRYPTED</span>
<span class="px-2 py-0.5 bg-terminal-green/20 text-terminal-green border border-terminal-green/30 text-[10px] font-bold rounded">VERIFIED</span>
</div>
</div>
</div>
<!-- Module 2: Agent Management -->
<div class="md:col-span-6 bg-white/5 border border-white/10 rounded-xl overflow-hidden group hover:border-primary/50 transition-all">
<div class="p-4 border-b border-white/10 flex justify-between items-center bg-white/5">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-primary">robot_2</span>
<h3 class="font-bold text-white">Agent Fleet Status</h3>
</div>
<span class="text-[10px] px-2 py-0.5 rounded-full bg-white/10 text-slate-300 border border-white/20 uppercase tracking-widest font-black">Management</span>
</div>
<div class="p-0">
<table class="w-full text-left text-sm">
<thead class="bg-white/5 text-slate-500 text-[10px] uppercase tracking-widest font-bold">
<tr>
<th class="px-6 py-3">Agent ID</th>
<th class="px-6 py-3">Role</th>
<th class="px-6 py-3">Status</th>
<th class="px-6 py-3 text-right">Activity</th>
</tr>
</thead>
<tbody class="divide-y divide-white/5">
<tr class="hover:bg-white/5 transition-colors">
<td class="px-6 py-4 font-mono text-xs text-white">SENTINEL_ALPHA</td>
<td class="px-6 py-4 text-slate-400">Guardian</td>
<td class="px-6 py-4"><span class="flex items-center gap-1.5 text-terminal-green"><span class="size-1.5 rounded-full bg-terminal-green"></span> Running</span></td>
<td class="px-6 py-4 text-right">
<div class="flex justify-end gap-1">
<div class="h-4 w-1 bg-primary/40 rounded-full"></div>
<div class="h-4 w-1 bg-primary/60 rounded-full"></div>
<div class="h-4 w-1 bg-primary rounded-full"></div>
<div class="h-4 w-1 bg-primary/20 rounded-full"></div>
</div>
</td>
</tr>
<tr class="hover:bg-white/5 transition-colors">
<td class="px-6 py-4 font-mono text-xs text-white">BUILDER_OMEGA</td>
<td class="px-6 py-4 text-slate-400">Architect</td>
<td class="px-6 py-4"><span class="flex items-center gap-1.5 text-slate-500"><span class="size-1.5 rounded-full bg-slate-500"></span> Idle</span></td>
<td class="px-6 py-4 text-right">
<div class="flex justify-end gap-1">
<div class="h-4 w-1 bg-white/5 rounded-full"></div>
<div class="h-4 w-1 bg-white/5 rounded-full"></div>
<div class="h-4 w-1 bg-white/5 rounded-full"></div>
<div class="h-4 w-1 bg-white/5 rounded-full"></div>
</div>
</td>
</tr>
<tr class="hover:bg-white/5 transition-colors">
<td class="px-6 py-4 font-mono text-xs text-white">DESK_UNIT_04</td>
<td class="px-6 py-4 text-slate-400">Router</td>
<td class="px-6 py-4"><span class="flex items-center gap-1.5 text-terminal-green"><span class="size-1.5 rounded-full bg-terminal-green"></span> Running</span></td>
<td class="px-6 py-4 text-right">
<div class="flex justify-end gap-1">
<div class="h-4 w-1 bg-primary/30 rounded-full"></div>
<div class="h-4 w-1 bg-primary/80 rounded-full"></div>
<div class="h-4 w-1 bg-primary/10 rounded-full"></div>
<div class="h-4 w-1 bg-primary/60 rounded-full"></div>
</div>
</td>
</tr>
</tbody>
</table>
</div>
</div>
<!-- Module 3: Workflow Builder -->
<div class="md:col-span-6 bg-white/5 border border-white/10 rounded-xl overflow-hidden group hover:border-primary/50 transition-all">
<div class="p-4 border-b border-white/10 flex justify-between items-center bg-white/5">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-primary">account_tree</span>
<h3 class="font-bold text-white">Pipeline Pulse</h3>
</div>
<span class="text-[10px] px-2 py-0.5 rounded-full bg-white/10 text-slate-300 border border-white/20 uppercase tracking-widest font-black">Workflows</span>
</div>
<div class="p-6">
<div class="space-y-4">
<div class="flex items-center gap-4 p-3 rounded-lg bg-background-dark border border-white/5">
<div class="size-10 rounded bg-primary/10 border border-primary/30 flex items-center justify-center">
<span class="material-symbols-outlined text-primary">auto_mode</span>
</div>
<div class="flex-1">
<div class="flex justify-between items-center">
<h4 class="text-sm font-bold text-white">Auto-Heal Protocol v2</h4>
<span class="text-[10px] text-slate-500 font-mono">ID: 9823-X</span>
</div>
<div class="w-full bg-white/5 h-1 rounded-full mt-2">
<div class="bg-primary h-full rounded-full" style="width: 75%"></div>
</div>
</div>
<span class="text-xs text-primary font-bold">75%</span>
</div>
<div class="flex items-center gap-4 p-3 rounded-lg bg-background-dark border border-white/5">
<div class="size-10 rounded bg-slate-800 border border-white/10 flex items-center justify-center">
<span class="material-symbols-outlined text-slate-400">cycle</span>
</div>
<div class="flex-1">
<div class="flex justify-between items-center">
<h4 class="text-sm font-bold text-white">Daily Summary Compilation</h4>
<span class="text-[10px] text-slate-500 font-mono">ID: 1042-Q</span>
</div>
<div class="w-full bg-white/5 h-1 rounded-full mt-2">
<div class="bg-slate-700 h-full rounded-full" style="width: 100%"></div>
</div>
</div>
<span class="text-xs text-slate-500 font-bold">Done</span>
</div>
<button class="w-full py-2 border border-dashed border-white/10 rounded-lg text-xs text-slate-500 hover:text-white hover:border-white/20 transition-all font-bold">
                            VIEW ALL ACTIVE PIPELINES
                        </button>
</div>
</div>
</div>
<!-- Module 4: Agent Logs (Live Terminal Feed) -->
<div class="md:col-span-12 bg-white/5 border border-white/10 rounded-xl overflow-hidden group hover:border-primary/50 transition-all">
<div class="p-4 border-b border-white/10 flex justify-between items-center bg-white/5">
<div class="flex items-center gap-2">
<span class="material-symbols-outlined text-primary">terminal</span>
<h3 class="font-bold text-white">Execution Truth</h3>
<span class="flex items-center gap-1.5 text-xs text-terminal-green ml-4">
<span class="size-1.5 rounded-full bg-terminal-green animate-ping"></span> Live Feed
                        </span>
</div>
<div class="flex items-center gap-4">
<div class="flex gap-2">
<span class="px-2 py-0.5 bg-slate-800 rounded text-[10px] font-bold text-slate-400">INFO</span>
<span class="px-2 py-0.5 bg-slate-800 rounded text-[10px] font-bold text-slate-400">WARN</span>
<span class="px-2 py-0.5 bg-slate-800 rounded text-[10px] font-bold text-slate-400">ERROR</span>
</div>
<button class="text-slate-400 hover:text-white"><span class="material-symbols-outlined text-xl">file_download</span></button>
</div>
</div>
<div class="p-6 terminal-bg font-mono text-[13px] leading-relaxed overflow-x-auto">
<div class="text-slate-400"><span class="text-slate-600">[2024-05-24 14:22:01]</span> <span class="text-primary">INFO:</span> Agent <span class="text-white">SENTINEL_ALPHA</span> initiated security sweep...</div>
<div class="text-slate-400"><span class="text-slate-600">[2024-05-24 14:22:03]</span> <span class="text-primary">INFO:</span> No perimeter breaches detected.</div>
<div class="text-slate-400"><span class="text-slate-600">[2024-05-24 14:22:05]</span> <span class="text-warning">WARN:</span> Node SA_DIST_04 reporting elevated latency (56ms). Re-routing active task...</div>
<div class="text-slate-400"><span class="text-slate-600">[2024-05-24 14:22:06]</span> <span class="text-primary">INFO:</span> Task <span class="text-white">WKF_9823-X</span> migration successful.</div>
<div class="text-slate-400"><span class="text-slate-600">[2024-05-24 14:22:08]</span> <span class="text-terminal-green">SUCCESS:</span> Agent <span class="text-white">BUILDER_OMEGA</span> completed resource synthesis.</div>
<div class="text-slate-200 mt-2 animate-pulse">_</div>
</div>
</div>
</div>
<!-- Integration Health Matrix -->
<div class="mt-8 p-6 bg-white/5 border border-white/10 rounded-xl">
<h3 class="font-bold text-white mb-6 flex items-center gap-2">
<span class="material-symbols-outlined text-primary">link</span> Integration Health Matrix
            </h3>
<div class="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-6">
<div class="flex flex-col items-center gap-3">
<div class="size-12 rounded-full border border-terminal-green/30 bg-terminal-green/10 flex items-center justify-center text-terminal-green">
<span class="material-symbols-outlined">sync_alt</span>
</div>
<div class="text-center">
<p class="text-[10px] text-slate-500 font-bold uppercase">Agents → Logs</p>
<p class="text-xs text-white font-medium">CONNECTED</p>
</div>
</div>
<div class="flex flex-col items-center gap-3">
<div class="size-12 rounded-full border border-terminal-green/30 bg-terminal-green/10 flex items-center justify-center text-terminal-green">
<span class="material-symbols-outlined">account_tree</span>
</div>
<div class="text-center">
<p class="text-[10px] text-slate-500 font-bold uppercase">Agents → WKF</p>
<p class="text-xs text-white font-medium">SYNCHRONIZED</p>
</div>
</div>
<div class="flex flex-col items-center gap-3">
<div class="size-12 rounded-full border border-terminal-green/30 bg-terminal-green/10 flex items-center justify-center text-terminal-green">
<span class="material-symbols-outlined">cloud_done</span>
</div>
<div class="text-center">
<p class="text-[10px] text-slate-500 font-bold uppercase">API Endpoint</p>
<p class="text-xs text-white font-medium">STABLE</p>
</div>
</div>
<div class="flex flex-col items-center gap-3 opacity-50">
<div class="size-12 rounded-full border border-slate-600 bg-white/5 flex items-center justify-center text-slate-500">
<span class="material-symbols-outlined">memory</span>
</div>
<div class="text-center">
<p class="text-[10px] text-slate-500 font-bold uppercase">GPU CLUSTER</p>
<p class="text-xs text-slate-500 font-medium">OFFLINE</p>
</div>
</div>
<div class="flex flex-col items-center gap-3">
<div class="size-12 rounded-full border border-terminal-green/30 bg-terminal-green/10 flex items-center justify-center text-terminal-green">
<span class="material-symbols-outlined">verified_user</span>
</div>
<div class="text-center">
<p class="text-[10px] text-slate-500 font-bold uppercase">Auth Provider</p>
<p class="text-xs text-white font-medium">VERIFIED</p>
</div>
</div>
<div class="flex flex-col items-center gap-3">
<div class="size-12 rounded-full border border-terminal-green/30 bg-terminal-green/10 flex items-center justify-center text-terminal-green">
<span class="material-symbols-outlined">database</span>
</div>
<div class="text-center">
<p class="text-[10px] text-slate-500 font-bold uppercase">Log Database</p>
<p class="text-xs text-white font-medium">ACTIVE</p>
</div>
</div>
</div>
</div>
</main>
<!-- Footer Meta -->
<footer class="p-6 border-t border-white/5 bg-background-dark/50 text-slate-500 text-[10px] font-mono tracking-widest uppercase flex justify-between items-center">
<div>© 2024 SOVEREIGN AGENT SYSTEMS - MISSION CONTROL LAYER</div>
<div class="flex gap-4">
<span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-terminal-green"></span> GLOBAL_SYNC_OK</span>
<span class="flex items-center gap-1"><span class="size-1.5 rounded-full bg-primary"></span> SESSION_ACTIVE</span>
</div>
</footer>
</div>
</body></html>
