// Adapter for the first-party TV site, using the same native media decoders.
// No URL extraction, ad filtering or private account data reads.
(() => {
 const video=()=>document.querySelector("video");
 const player=()=>document.querySelector("ytlr-player")?.__instance?.J;
 const advertising=()=>player()?.isAdActive===true;
 window.youtubeStatus={ready:false,state:-1,error:0,site:true};
 window.youtubeTVLoad=(id,start,play)=>{
  window.__youtubeTVRequest={id,start,play};
  location.hash="/watch?v="+encodeURIComponent(id);
 };
 window.youtubePlayer={
  getCurrentTime(){return video()?.currentTime||0;},
  getDuration(){const d=video()?.duration;return !advertising()&&Number.isFinite(d)?d:0;},
  playVideo(){player()?.play?.();const v=video();if(v)Promise.resolve(v.play()).catch(()=>{window.youtubeStatus.error=-1;});},
  pauseVideo(){player()?.pause?.();video()?.pause();},
  seekTo(t){const v=video();if(v&&!advertising())v.currentTime=t;}
 };
 let applied=false,ended=false;
 document.addEventListener("ended",e=>{if(e.target===video()&&!advertising())ended=true},true);
 setInterval(()=>{
  const v=video(),s=window.youtubeStatus,request=window.__youtubeTVRequest;
  if(!request){if(v&&!v.paused)v.pause();return;} // never autoplay TV home recommendations
  s.ready=!!v;
  if(!v)return;
  if(!applied&&v.readyState>=1&&!advertising()){
   applied=true;
   if(request.start>0)v.currentTime=request.start;
   if(request.play)window.youtubePlayer.playVideo();else window.youtubePlayer.pauseVideo();
  }
  if(v.ended&&!advertising())ended=true; // latch EOF before the site's autonext
  if(ended)v.pause();
  s.error=v.error?5:0;
  s.state=ended?0:v.paused?2:v.readyState<2?3:1;
 },250);
})();
