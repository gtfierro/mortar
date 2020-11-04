// automatic thebe start
window.onload = function(){


    // var cfg = {
    //     bootstrap: true,
    //     kernelOptions: {},
    // };
    // cfg.kernelOptions.serverSettings = {
    //   "baseUrl": "http://127.0.0.1:8888",
    //   "token": "test-secret"
    // };
    // console.log("updating thebe with config", cfg);
    // thebelab.bootstrap(cfg);


    // console.log("rendering")
    // var cells = thebelab.renderAllCells();
    // thebelab.requestKernel(cfg.kernelOptions)
    //         .then(function (kernel) { thebelab.hookupKernel(kernel, cells) });


    //console.log("rewrite config", document.querySelector('script[type="text/x-thebe-config"]'));
    //var cfg = document.querySelector('script[type="text/x-thebe-config"]');
    //console.log("config", cfg.innerText);
    //if (cfg != null) {
    //    cfg.innerText = '{bootstrap: true, kernelOptions: { name: "python3", serverSettings: { "baseUrl": "http://127.0.0.1:8888", "token": "test-secret" } }}';
    //    //thebelab.bootstrap();
    //    //
    //}
    //console.log("config", cfg.innerText);
    //thebelab.bootstrap();

    if(document.getElementsByClassName('cell_input').length > 0 ){
        initThebeSBT();
    }
}
