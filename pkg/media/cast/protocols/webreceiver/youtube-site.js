// Bridge the official full YouTube website to the existing media controls.
// No extraction of stream URLs, cookie import, sign-in automation or ad removal.
(() => {
  const video = () => document.querySelector("video");
  const advertising = () => !!document.getElementById("movie_player")?.classList.contains("ad-showing");
  window.youtubeStatus = {ready:false,state:-1,error:0,site:true};
  window.youtubePlayer = {
    getCurrentTime() { return video()?.currentTime || 0; },
    getDuration() { const d=video()?.duration; return !advertising() && Number.isFinite(d) ? d : 0; },
    playVideo() { const v=video(); if(v) Promise.resolve(v.play()).catch(()=>{window.youtubeStatus.error=-1;}); },
    pauseVideo() { video()?.pause(); },
    seekTo(t) { const v=video(); if(v && !advertising()) v.currentTime=t; }
  };
  let initialStateApplied=false;
  const update = () => {
    const v=video(), s=window.youtubeStatus;
    s.ready=!!v;
    if(!v) return;
    if(!initialStateApplied && v.readyState>=1) {
      initialStateApplied=true;
      if(new URLSearchParams(location.search).get("autoplay")==="0") v.pause();
    }
    s.error=v.error ? 5 : 0;
    s.state=v.ended && !advertising() ? 0 : v.paused ? 2 : v.readyState<2 ? 3 : 1;
  };
  setInterval(update,250);
})();
