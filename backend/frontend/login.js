(function(){
  var t=localStorage.getItem('dockgraph-theme');
  if(t==='light')document.documentElement.setAttribute('data-theme','light');

  var f=document.getElementById('f'),p=document.getElementById('p'),
      b=document.getElementById('b'),e=document.getElementById('e');

  f.addEventListener('submit',function(ev){
    ev.preventDefault();
    e.textContent='';
    b.disabled=true;
    fetch('/api/login',{
      method:'POST',
      headers:{'Content-Type':'application/json'},
      body:JSON.stringify({password:p.value})
    }).then(function(res){
      if(res.ok){window.location.href='/';}
      else if(res.status===429){
        return res.json().then(function(d){
          e.textContent='Too many attempts. Try again in '+d.retryAfter+'s.';
        });
      }else if(res.status===503){e.textContent='Server busy. Try again in a moment.';}
      else{e.textContent='Invalid password';}
    }).catch(function(){
      e.textContent='Connection error';
    }).finally(function(){b.disabled=false;});
  });
})();
