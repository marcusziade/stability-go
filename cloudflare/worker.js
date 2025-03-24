// worker.js - Cloudflare Worker wrapper for TinyGo WASM
let instance;

// Initialize the WASM module
async function initWasm(env) {
  // Load the WASM module
  const wasmModule = fetch('worker.wasm');
  
  // Import object for the WASM module
  const importObject = {
    env: {
      // Provide environment variables to the Go code
      ...env,
      
      // Memory management helpers
      memory: new WebAssembly.Memory({ initial: 10, maximum: 100 }),
      
      // Optional: Add KV storage functions that can be called from Go
      kv_get: async (keyPtr, keyLen) => {
        const key = new TextDecoder().decode(
          new Uint8Array(instance.exports.memory.buffer, keyPtr, keyLen)
        );
        
        if (typeof STABILITY_CACHE !== 'undefined') {
          try {
            const value = await STABILITY_CACHE.get(key);
            return value || '';
          } catch (error) {
            console.error('KV get error:', error);
            return '';
          }
        }
        return '';
      },
      
      kv_put: async (keyPtr, keyLen, valuePtr, valueLen, ttl) => {
        const key = new TextDecoder().decode(
          new Uint8Array(instance.exports.memory.buffer, keyPtr, keyLen)
        );
        
        const value = new TextDecoder().decode(
          new Uint8Array(instance.exports.memory.buffer, valuePtr, valueLen)
        );
        
        if (typeof STABILITY_CACHE !== 'undefined') {
          try {
            const options = ttl > 0 ? { expirationTtl: ttl } : {};
            await STABILITY_CACHE.put(key, value, options);
            return true;
          } catch (error) {
            console.error('KV put error:', error);
            return false;
          }
        }
        return false;
      }
    },
    wasi_snapshot_preview1: {
      // Minimal WASI implementation
      fd_write: () => 0,
      fd_close: () => 0,
      fd_seek: () => 0,
      proc_exit: () => 0,
    }
  };
  
  // Instantiate the WASM module
  const { instance: wasmInstance } = await WebAssembly.instantiateStreaming(wasmModule, importObject);
  instance = wasmInstance;
  
  // Return the instance
  return wasmInstance;
}

// Main event listener for Cloudflare Worker
addEventListener('fetch', event => {
  event.respondWith(handleRequest(event.request));
});

// Handle an incoming request
async function handleRequest(request) {
  // Initialize WASM if not already done
  if (!instance) {
    try {
      // Pass environment variables to the WASM module
      const env = {
        STABILITY_API_KEY: STABILITY_API_KEY || '',
      };
      
      await initWasm(env);
    } catch (error) {
      return new Response(`Failed to initialize WASM: ${error.message}`, { status: 500 });
    }
  }
  
  try {
    // Convert the request to a format our Go code can work with
    const reqData = await serializeRequest(request);
    
    // Allocate memory for the request in the WASM module
    const reqPtr = instance.exports.alloc(reqData.length);
    
    // Copy the request data to WASM memory
    const memory = new Uint8Array(instance.exports.memory.buffer);
    memory.set(reqData, reqPtr);
    
    // Create pointers for the response
    const respPtrPtr = instance.exports.alloc(4); // 4 bytes for uint32
    const respLenPtr = instance.exports.alloc(4); // 4 bytes for uint32
    
    // Initialize response pointers to 0
    new Uint32Array(instance.exports.memory.buffer, respPtrPtr, 1)[0] = 0;
    new Uint32Array(instance.exports.memory.buffer, respLenPtr, 1)[0] = 0;
    
    // Call the HandleRequest function
    instance.exports.HandleRequest(reqPtr, reqData.length, respPtrPtr, respLenPtr);
    
    // Get the response pointer and length
    const respPtr = new Uint32Array(instance.exports.memory.buffer, respPtrPtr, 1)[0];
    const respLen = new Uint32Array(instance.exports.memory.buffer, respLenPtr, 1)[0];
    
    // Get the response data
    const respData = new Uint8Array(instance.exports.memory.buffer, respPtr, respLen);
    
    // Parse the response
    const resp = JSON.parse(new TextDecoder().decode(respData));
    
    // Create and return the Response object
    return new Response(resp.Body, {
      status: resp.StatusCode,
      headers: resp.Headers
    });
  } catch (error) {
    return new Response(`Error: ${error.message}`, { status: 500 });
  }
}

// Serialize a Request object to a format our Go code can work with
async function serializeRequest(request) {
  // Get the request body as bytes
  const body = await request.arrayBuffer().then(
    buffer => new Uint8Array(buffer)
  );
  
  // Convert headers to a format Go can work with
  const headers = {};
  for (const [key, value] of request.headers.entries()) {
    headers[key] = [value];
  }
  
  // Create the request object
  const req = {
    method: request.method,
    url: request.url,
    headers: headers,
    body: Array.from(body) // Convert to regular array for JSON serialization
  };
  
  // Serialize the request
  return new TextEncoder().encode(JSON.stringify(req));
}