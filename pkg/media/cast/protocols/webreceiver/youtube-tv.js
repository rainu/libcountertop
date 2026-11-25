// Adapter for the first-party TV site, using the same native media decoders.
// No URL extraction, ad filtering or private account data reads.
(() => {
 const video=()=>document.querySelector("video");
 const player=()=>document.querySelector("ytlr-player")?.__instance?.J;
 const advertising=()=>player()?.isAdActive===true;
 let desiredPlay=null;
 window.youtubeStatus={ready:false,state:-1,error:0,site:true};
 window.youtubeTVLoad=(id,start,play)=>{
  window.__youtubeTVRequest={id,start,play,at:Date.now(),lastRoute:Date.now(),routes:1};
  location.hash="/watch?v="+encodeURIComponent(id);
 };
 window.youtubePlayer={
  getCurrentTime(){return video()?.currentTime||0;},
  getDuration(){const d=video()?.duration;return !advertising()&&Number.isFinite(d)?d:0;},
  playVideo(){desiredPlay=true;if(window.__youtubeTVRequest)window.__youtubeTVRequest.play=true;player()?.play?.();const v=video();if(v&&v.currentSrc)Promise.resolve(v.play()).catch(()=>{window.youtubeStatus.error=-1;});},
  pauseVideo(){desiredPlay=false;if(window.__youtubeTVRequest)window.__youtubeTVRequest.play=false;player()?.pause?.();video()?.pause();},
  seekTo(t){const v=video();if(v&&!advertising()){v.currentTime=t;if(desiredPlay===false)v.pause();}}
 };
 let applied=false,ended=false,kicked=0;
 document.addEventListener("play",e=>{if(e.target===video()&&desiredPlay===false)video()?.pause()},true);
 document.addEventListener("ended",e=>{if(e.target===video()&&!advertising())ended=true},true);
 setInterval(()=>{
  const v=video(),s=window.youtubeStatus,request=window.__youtubeTVRequest;
  if(!request){if(v&&!v.paused)v.pause();return;} // never autoplay TV home recommendations
  s.ready=!!v;
  if(!v)return;
  if(!applied&&!v.currentSrc&&Date.now()-request.lastRoute>8000&&request.routes<3){
   request.lastRoute=Date.now();request.routes++;
   location.hash="";
   setTimeout(()=>location.hash="/watch?v="+encodeURIComponent(request.id),250);
  }
  // The TV application must initiate its first load before decoder metadata
  // exists. Waiting for readyState first deadlocks startup on the real device.
  if(!applied&&v.readyState<1&&Date.now()-kicked>2000){
   kicked=Date.now();player()?.play?.();
  }
  if(!applied&&v.readyState>=1&&!advertising()){
   applied=true;
   if(request.start>0)v.currentTime=request.start;
   if(request.play)window.youtubePlayer.playVideo();else window.youtubePlayer.pauseVideo();
  }
  if(v.ended&&!advertising())ended=true; // latch EOF before the site's autonext
  if(ended||(applied&&desiredPlay===false&&!v.paused))v.pause();
  s.error=v.error?5:0;
  s.state=ended?0:v.paused?2:v.readyState<2?3:1;
 },250);
})();
