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
      FileReader: "readonly"                                                                                                                                                
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
