  const browserGlobals = {
    globals: {
      ClipboardJS: "readonly",
      navigator: "readonly",
      window: "readonly",
      document: "readonly",
      URLSearchParams: "readonly",
      URL: "readonly",
      Blob: "readonly",
      Uint8Array: "readonly",
      atob: "readonly",
      fetch: "readonly",
      console: "readonly",
      FormData: "readonly",
      FileReader: "readonly",
      // $ and $$ are declared by utils.js and consumed as globals by the
      // classic-script pages (index.js, getmsg.js load it before them).
      // Once the ES-module rewrite lands, imports satisfy no-undef instead
      // — declaring the globals stays correct in both layouts.
      $: "readonly",
      $$: "readonly",
      alert: "readonly"
    }
  };                                                                                                                                                                        
                                                                                                                                                                            
  export default [                                                                                                                                                          
    {                                                                                                                                                                       
      files: ["web/static/**/*.js"],                                                                                                                                        
      ignores: ["web/static/clipboard-*js"],
      languageOptions: {                                                                                                                                                    
        sourceType: "module",                                                                                                                                               
        globals: { ...browserGlobals.globals }                                                                                                                              
      },                                                                                                                                                                    
      rules: {                                                                                                                                                              
        "no-undef": "error",                                                                                                                                                
        "no-unused-vars": "warn"                                                                                                                                            
      }                                                                                                                                                                     
    }                                                                                                                                                                       
  ];
