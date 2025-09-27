#!/usr/bin/env python3

import tkinter as tk
from tkinter import ttk, filedialog, messagebox, scrolledtext
import threading
import queue
import json
import os
import sys
import http.server
import socketserver
from datetime import datetime
import webbrowser

# Add the core directory to the path so we can import sniplicity
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'core'))

try:
    from core import sniplicity
except ImportError:
    import sniplicity

class SniplicilyApp:
    def __init__(self, root):
        self.root = root
        self.root.title("Sniplicity Desktop")
        self.root.geometry("800x600")
        
        # Configuration
        self.config_file = os.path.join(os.path.dirname(__file__), "config.json")
        self.config = self.load_config()
        
        # State variables
        self.input_dir = tk.StringVar(value=self.config.get("input_dir", ""))
        self.output_dir = tk.StringVar(value=self.config.get("output_dir", ""))
        self.is_watching = False
        self.is_server_running = False
        self.watch_thread = None
        self.server_thread = None
        self.server = None
        self.log_queue = queue.Queue()
        
        # Setup GUI
        self.setup_ui()
        
        # Start log processor
        self.process_log_queue()
        
        # Handle window closing
        self.root.protocol("WM_DELETE_WINDOW", self.on_closing)
        
    def load_config(self):
        """Load configuration from JSON file"""
        try:
            if os.path.exists(self.config_file):
                with open(self.config_file, 'r') as f:
                    return json.load(f)
        except Exception as e:
            self.log(f"Error loading config: {e}")
        return {}
    
    def save_config(self):
        """Save configuration to JSON file"""
        try:
            config = {
                "input_dir": self.input_dir.get(),
                "output_dir": self.output_dir.get()
            }
            with open(self.config_file, 'w') as f:
                json.dump(config, f, indent=2)
        except Exception as e:
            self.log(f"Error saving config: {e}")
    
    def setup_ui(self):
        """Create the main user interface"""
        
        # Main frame
        main_frame = ttk.Frame(self.root, padding="10")
        main_frame.grid(row=0, column=0, sticky=(tk.W, tk.E, tk.N, tk.S))
        
        # Configure grid weights
        self.root.columnconfigure(0, weight=1)
        self.root.rowconfigure(0, weight=1)
        main_frame.columnconfigure(1, weight=1)
        main_frame.rowconfigure(4, weight=1)
        
        # Input directory selection
        ttk.Label(main_frame, text="Input Directory:").grid(row=0, column=0, sticky=tk.W, pady=(0, 5))
        
        input_frame = ttk.Frame(main_frame)
        input_frame.grid(row=0, column=1, columnspan=2, sticky=(tk.W, tk.E), pady=(0, 5))
        input_frame.columnconfigure(0, weight=1)
        
        self.input_entry = ttk.Entry(input_frame, textvariable=self.input_dir, state="readonly")
        self.input_entry.grid(row=0, column=0, sticky=(tk.W, tk.E), padx=(0, 5))
        
        ttk.Button(input_frame, text="Browse...", command=self.browse_input_dir).grid(row=0, column=1)
        
        # Output directory selection
        ttk.Label(main_frame, text="Output Directory:").grid(row=1, column=0, sticky=tk.W, pady=(0, 5))
        
        output_frame = ttk.Frame(main_frame)
        output_frame.grid(row=1, column=1, columnspan=2, sticky=(tk.W, tk.E), pady=(0, 5))
        output_frame.columnconfigure(0, weight=1)
        
        self.output_entry = ttk.Entry(output_frame, textvariable=self.output_dir, state="readonly")
        self.output_entry.grid(row=0, column=0, sticky=(tk.W, tk.E), padx=(0, 5))
        
        ttk.Button(output_frame, text="Browse...", command=self.browse_output_dir).grid(row=0, column=1)
        
        # Control buttons
        button_frame = ttk.Frame(main_frame)
        button_frame.grid(row=2, column=0, columnspan=3, pady=(10, 0), sticky=(tk.W, tk.E))
        
        self.watch_button = ttk.Button(button_frame, text="Start Watching", command=self.toggle_watching)
        self.watch_button.grid(row=0, column=0, padx=(0, 5))
        
        self.build_button = ttk.Button(button_frame, text="Build Once", command=self.build_once)
        self.build_button.grid(row=0, column=1, padx=(0, 5))
        
        self.server_button = ttk.Button(button_frame, text="Start Server", command=self.toggle_server)
        self.server_button.grid(row=0, column=2, padx=(0, 5))
        
        self.view_button = ttk.Button(button_frame, text="View Site", command=self.view_site, state="disabled")
        self.view_button.grid(row=0, column=3, padx=(0, 5))
        
        # Status bar
        status_frame = ttk.Frame(main_frame)
        status_frame.grid(row=3, column=0, columnspan=3, pady=(10, 0), sticky=(tk.W, tk.E))
        status_frame.columnconfigure(0, weight=1)
        
        self.status_label = ttk.Label(status_frame, text="Ready")
        self.status_label.grid(row=0, column=0, sticky=tk.W)
        
        self.server_status_label = ttk.Label(status_frame, text="Server: Stopped")
        self.server_status_label.grid(row=0, column=1, sticky=tk.E)
        
        # Log area
        ttk.Label(main_frame, text="Activity Log:").grid(row=4, column=0, sticky=(tk.W, tk.N), pady=(10, 5))
        
        log_frame = ttk.Frame(main_frame)
        log_frame.grid(row=4, column=0, columnspan=3, pady=(10, 0), sticky=(tk.W, tk.E, tk.N, tk.S))
        log_frame.columnconfigure(0, weight=1)
        log_frame.rowconfigure(0, weight=1)
        
        self.log_text = scrolledtext.ScrolledText(log_frame, height=15, state="disabled")
        self.log_text.grid(row=0, column=0, sticky=(tk.W, tk.E, tk.N, tk.S))
        
        # Clear log button
        ttk.Button(log_frame, text="Clear Log", command=self.clear_log).grid(row=1, column=0, pady=(5, 0), sticky=tk.E)
        
        self.log("Sniplicity Desktop started")
        if self.input_dir.get() and self.output_dir.get():
            self.log(f"Loaded configuration: {self.input_dir.get()} -> {self.output_dir.get()}")
    
    def browse_input_dir(self):
        """Browse for input directory"""
        directory = filedialog.askdirectory(
            title="Select Input Directory",
            initialdir=self.input_dir.get() or os.getcwd()
        )
        if directory:
            self.input_dir.set(directory)
            self.save_config()
            self.log(f"Input directory set to: {directory}")
    
    def browse_output_dir(self):
        """Browse for output directory"""
        directory = filedialog.askdirectory(
            title="Select Output Directory",
            initialdir=self.output_dir.get() or os.getcwd()
        )
        if directory:
            self.output_dir.set(directory)
            self.save_config()
            self.log(f"Output directory set to: {directory}")
    
    def log(self, message):
        """Add a message to the log queue"""
        timestamp = datetime.now().strftime("%H:%M:%S")
        self.log_queue.put(f"[{timestamp}] {message}")
    
    def process_log_queue(self):
        """Process messages from the log queue"""
        try:
            while True:
                message = self.log_queue.get_nowait()
                self.log_text.config(state="normal")
                self.log_text.insert(tk.END, message + "\\n")
                self.log_text.see(tk.END)
                self.log_text.config(state="disabled")
        except queue.Empty:
            pass
        
        # Schedule next check
        self.root.after(100, self.process_log_queue)
    
    def clear_log(self):
        """Clear the log display"""
        self.log_text.config(state="normal")
        self.log_text.delete(1.0, tk.END)
        self.log_text.config(state="disabled")
    
    def validate_directories(self):
        """Validate that input and output directories are set"""
        if not self.input_dir.get():
            messagebox.showerror("Error", "Please select an input directory")
            return False
        if not self.output_dir.get():
            messagebox.showerror("Error", "Please select an output directory")
            return False
        if not os.path.exists(self.input_dir.get()):
            messagebox.showerror("Error", f"Input directory does not exist: {self.input_dir.get()}")
            return False
        return True
    
    def build_once(self):
        """Build the site once without watching"""
        if not self.validate_directories():
            return
            
        def build_thread():
            try:
                self.log("Starting single build...")
                self.status_label.config(text="Building...")
                
                # Capture sniplicity output
                import io
                from contextlib import redirect_stdout, redirect_stderr
                
                output_buffer = io.StringIO()
                
                with redirect_stdout(output_buffer), redirect_stderr(output_buffer):
                    sniplicity.build(self.input_dir.get(), self.output_dir.get(), False)
                
                output = output_buffer.getvalue()
                if output:
                    for line in output.split('\\n'):
                        if line.strip():
                            self.log(line.strip())
                
                self.log("Build completed successfully")
                self.status_label.config(text="Build completed")
                
            except Exception as e:
                self.log(f"Build error: {str(e)}")
                self.status_label.config(text="Build failed")
        
        threading.Thread(target=build_thread, daemon=True).start()
    
    def toggle_watching(self):
        """Start or stop watching for file changes"""
        if self.is_watching:
            self.stop_watching()
        else:
            self.start_watching()
    
    def start_watching(self):
        """Start watching for file changes"""
        if not self.validate_directories():
            return
            
        def watch_thread():
            try:
                self.log("Starting file watcher...")
                self.status_label.config(text="Watching for changes...")
                
                # Use a custom observer to capture output
                import io
                from contextlib import redirect_stdout, redirect_stderr
                
                output_buffer = io.StringIO()
                
                with redirect_stdout(output_buffer), redirect_stderr(output_buffer):
                    sniplicity.build(self.input_dir.get(), self.output_dir.get(), True)
                
            except Exception as e:
                self.log(f"Watch error: {str(e)}")
                self.status_label.config(text="Watch failed")
        
        self.is_watching = True
        self.watch_button.config(text="Stop Watching")
        self.watch_thread = threading.Thread(target=watch_thread, daemon=True)
        self.watch_thread.start()
    
    def stop_watching(self):
        """Stop watching for file changes"""
        self.is_watching = False
        self.watch_button.config(text="Start Watching")
        self.status_label.config(text="Stopped watching")
        self.log("Stopped watching for changes")
    
    def toggle_server(self):
        """Start or stop the web server"""
        if self.is_server_running:
            self.stop_server()
        else:
            self.start_server()
    
    def start_server(self):
        """Start the web server"""
        if not self.output_dir.get():
            messagebox.showerror("Error", "Please select an output directory first")
            return
        
        if not os.path.exists(self.output_dir.get()):
            messagebox.showerror("Error", f"Output directory does not exist: {self.output_dir.get()}")
            return
        
        def server_thread():
            try:
                os.chdir(self.output_dir.get())
                handler = http.server.SimpleHTTPRequestHandler
                
                with socketserver.TCPServer(("127.0.0.1", 3000), handler) as httpd:
                    self.server = httpd
                    self.log("Web server started at http://127.0.0.1:3000")
                    self.server_status_label.config(text="Server: Running on :3000")
                    self.view_button.config(state="normal")
                    httpd.serve_forever()
                    
            except OSError as e:
                if "Address already in use" in str(e):
                    self.log("Error: Port 3000 is already in use")
                    messagebox.showerror("Error", "Port 3000 is already in use by another application")
                else:
                    self.log(f"Server error: {str(e)}")
                    messagebox.showerror("Error", f"Failed to start server: {str(e)}")
                
                self.is_server_running = False
                self.server_button.config(text="Start Server")
                self.server_status_label.config(text="Server: Stopped")
                self.view_button.config(state="disabled")
        
        self.is_server_running = True
        self.server_button.config(text="Stop Server")
        self.server_thread = threading.Thread(target=server_thread, daemon=True)
        self.server_thread.start()
    
    def stop_server(self):
        """Stop the web server"""
        if self.server:
            self.server.shutdown()
            self.server = None
        
        self.is_server_running = False
        self.server_button.config(text="Start Server")
        self.server_status_label.config(text="Server: Stopped")
        self.view_button.config(state="disabled")
        self.log("Web server stopped")
    
    def view_site(self):
        """Open the site in the default web browser"""
        if self.is_server_running:
            webbrowser.open("http://127.0.0.1:3000")
            self.log("Opened site in browser")
    
    def on_closing(self):
        """Handle application closing"""
        if self.is_server_running:
            self.stop_server()
        
        self.save_config()
        self.root.destroy()

def main():
    root = tk.Tk()
    app = SniplicilyApp(root)
    root.mainloop()

if __name__ == "__main__":
    main()