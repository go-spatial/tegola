"""Read-only local process identity; command lines are never written to reports."""
import ctypes
import json
import os
from pathlib import Path
import subprocess


def process_identity(pid, binary, config, port):
    if pid <= 0:
        raise ValueError("Invalid fixture process ID")
    if os.name == "nt":
        script = ("[Console]::OutputEncoding=[Text.UTF8Encoding]::new(); "
                  f"Get-CimInstance Win32_Process -Filter 'ProcessId={pid}' | "
                  "Select-Object ExecutablePath,CommandLine,CreationDate | ConvertTo-Json -Compress")
        raw = subprocess.check_output(["powershell.exe", "-NoProfile", "-Command", script], timeout=30)
        info = json.loads(raw.decode("utf-8-sig"))
        executable = info["ExecutablePath"]
        creation = info["CreationDate"]
        listen_script = (f"@(Get-NetTCPConnection -OwningProcess {pid} -State Listen -ErrorAction SilentlyContinue | "
                         f"Where-Object LocalPort -EQ {port}).Count")
        if int(subprocess.check_output(["powershell.exe", "-NoProfile", "-Command", listen_script], timeout=30).strip()) < 1:
            raise ValueError("Fixture process does not own the target listening port")
        count = ctypes.c_int()
        shell = ctypes.WinDLL("shell32", use_last_error=True)
        shell.CommandLineToArgvW.restype = ctypes.POINTER(ctypes.c_wchar_p)
        shell.CommandLineToArgvW.argtypes = [ctypes.c_wchar_p, ctypes.POINTER(ctypes.c_int)]
        array = shell.CommandLineToArgvW(info["CommandLine"], ctypes.byref(count))
        if not array:
            raise ValueError("Cannot read fixture arguments")
        try:
            arguments = [array[i] for i in range(count.value)]
        finally:
            kernel = ctypes.WinDLL("kernel32", use_last_error=True)
            kernel.LocalFree.argtypes = [ctypes.c_void_p]
            kernel.LocalFree(array)
    elif Path("/proc").is_dir():
        executable = os.readlink(f"/proc/{pid}/exe")
        arguments = Path(f"/proc/{pid}/cmdline").read_bytes().decode().rstrip("\0").split("\0")
        creation = Path(f"/proc/{pid}/stat").read_text().rsplit(")", 1)[1].split()[19]
        sockets = {os.readlink(p) for p in Path(f"/proc/{pid}/fd").iterdir() if p.is_symlink()}
        listening = False
        for name in ("tcp", "tcp6"):
            for row in Path(f"/proc/{pid}/net/{name}").read_text().splitlines()[1:]:
                fields = row.split()
                if fields[3] == "0A" and int(fields[1].split(":")[1], 16) == port and f"socket:[{fields[9]}]" in sockets:
                    listening = True
        if not listening:
            raise ValueError("Fixture process does not own the target listening port")
    else:
        raise ValueError("Process provenance currently supports Windows and Linux")
    if Path(executable).resolve() != Path(binary).resolve():
        raise ValueError("Running executable differs from supplied binary")
    selected = []
    for i, argument in enumerate(arguments):
        if argument == "--config" and i + 1 < len(arguments):
            selected.append(arguments[i + 1])
        elif argument.startswith("--config="):
            selected.append(argument.partition("=")[2])
    if len(selected) != 1 or Path(selected[0]).resolve() != Path(config).resolve():
        raise ValueError("Running configuration differs from supplied config")
    return {"pid": pid, "created": creation, "port": port, "binary": str(Path(binary).resolve()), "config": str(Path(config).resolve()),
            "scope": "Executable path and explicit --config argument verified; full command line not retained"}
