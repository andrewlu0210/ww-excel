// Marker symbols use the same logical coordinates on screen and in PDF images.
export function drawChartMarker(ctx:CanvasRenderingContext2D,x:number,y:number,size:number,symbol='auto'):void {
 if(symbol==='none')return;
 ctx.beginPath();
 if(symbol==='square'){ctx.rect(x-size,y-size,size*2,size*2);}
 else if(symbol==='diamond'||symbol==='triangle'||symbol==='star'){
  const count=symbol==='star'?10:symbol==='triangle'?3:4;
  for(let i=0;i<count;i++){
   const radius=symbol==='star'&&i%2?size*.45:size,angle=-Math.PI/2+i*Math.PI*2/count;
   const px=x+Math.cos(angle)*radius,py=y+Math.sin(angle)*radius;
   if(i===0)ctx.moveTo(px,py);else ctx.lineTo(px,py);
  }
  ctx.closePath();
 }else if(['x','plus','dash'].includes(symbol)){
  ctx.save();ctx.strokeStyle=ctx.fillStyle;ctx.lineWidth=1.5;
  if(symbol==='x'){ctx.moveTo(x-size,y-size);ctx.lineTo(x+size,y+size);ctx.moveTo(x-size,y+size);ctx.lineTo(x+size,y-size);}
  else{ctx.moveTo(x-size,y);ctx.lineTo(x+size,y);if(symbol==='plus'){ctx.moveTo(x,y-size);ctx.lineTo(x,y+size);}}
  ctx.stroke();ctx.restore();return;
 }else{ctx.arc(x,y,symbol==='dot'?size*.5:size,0,Math.PI*2);}
 ctx.fill();
}
