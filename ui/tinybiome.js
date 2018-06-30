"use strict"

var servers;
var currentRoom;

var tileSize = 50;
var hidingBbox = true;
var debugMode = false;
var maxCamPad = 150;
var camPad = maxCamPad;
var maxQuality = 3;


var gfx = {};
var gfx_loaded = {"default.js":true}

DataView.prototype.getUTF8String = function(offset, length) {
    var utf16View = [];
    for (var i = 0; i < length; ++i) {
        utf16View.push( this.getUint8(offset + i) );
    }
    return ab2str(utf16View)
};

var ws;

function server(location) {
	var self = this;
	this.roomCount = 0
	this.ws = new WebSocket(location)
	this.rooms = {};
	this.ws.onopen = function() {
	}
	this.ws.onerror = function() {
		ohno("Websocket Error! Refresh in a bit, it might have been restarted...")
	}
	this.ws.onclose = function() {
		ohno("Websocket Closed?")
	}
	this.ws.onmessage = function(m) {
		var d = JSON.parse(m.data)
		if (d.meth=="add") {
			new sock(d.address);
		}
		if (d.meth=="rem") {
			var s = self.findServer(d.address)
			self.removeServer(s)
			s.remove()
		}
	}
}
server.prototype.addServer = function(sock) {
	this.servers.push(sock)
	this.ensureConnected()
}
server.prototype.findServer = function(location) {
	for(var i=0;i<this.servers;i+=1) {
		if (this.servers[i].location == location) {
			return this.servers[i]
		}
	}
}
server.prototype.removeServer = function(sock) {
	for(var i=0;i<this.servers;i+=1) {
		if (this.servers[i] == sock) {
			this.servers.splice(i, 1);
			break
		}
	}
	this.ensureConnected()
}
server.prototype.addRoom = function(room) {
	var name = ""+room.id+"@"+room.server.location;
	this.rooms[name] = room
	if (this.roomCount==0) {
		this.connect()
	}
	this.roomCount += 1
	document.getElementById("serversdown").style.display="none";
	room.ele = document.getElementById("servertemplate").cloneNode(true);
	document.getElementById("serverlist").appendChild(room.ele)

	room.ele.onclick = function() {
		servers.pickRoom(room)
	}
	room.refreshElement = function() {
		room.ele.querySelector(".title").innerHTML = nameFromRoom(room)
	}
	room.refreshElement()
}
server.prototype.connect = function() {
	console.info("PICKING A GOOD DEFAULT SERVER")
	for(var i in this.rooms) {
		console.info("PICKING",this.rooms[i])
		this.rooms[i].server.writeSpectate(this.rooms[i].id)
		break
	}
}
server.prototype.removeRoom = function(room) {
	console.info("REMOVING ROOM", room)
	var name = ""+room.id+"@"+room.server.location;

	delete this.rooms[name]

	this.roomCount -= 1
	if (room==currentRoom) {
		this.connect()
	}
	document.getElementById("serverlist").removeChild(room.ele)
	if (this.roomCount==0) {
		document.getElementById("serversdown").style.display="block";
	}
}
server.prototype.pickRoom = function(room) {
	room.server.writeSpectate(room.id)
}
function nameFromRoom(room) {
	return room.name+" (PING "+Math.floor(room.server.latency*5)/10+"ms, "+room.playercount+" PLAYERS)"
}


function sock(location) {
	console.log("NEW SERVER", this)
	this.room = null

	this.rooms = {}

	this.location = location
	this.messageHandlers = [
		this.handleNewRoom.bind(this),
		this.handleNewActor.bind(this),
		this.handleNewPellet.bind(this),
		this.handleRemovePellet.bind(this),
		this.handleNewPlayer.bind(this),
		this.handleRenamePlayer.bind(this),
		this.handleDestroyPlayer.bind(this),
		this.handleOwnPlayer.bind(this),
		this.handleRemoveActor.bind(this),
		this.handleMoveActor.bind(this),
		this.handleSetMass.bind(this),
		this.handleMultiPellet.bind(this),
		this.handleDescribeActor.bind(this),
		this.handleSync.bind(this),
		this.handlePong.bind(this),
		this.handleStopSpec.bind(this)]
	this.messageMap = []
	this.latency = 0;
	this.ws = new WebSocket("wss://"+location)
	this.lastPong = (new Date());

	var self = this;


	this.connected = false
	this.ws.binaryType = "arraybuffer";
	this.mytick = this.tick.bind(this)
	this.myremove = this.remove.bind(this)
	this.ws.onopen = function() {
		self.connected = true
		console.log("CONNECTED TO",location)
		self.mytick()
		self.start.apply(self)
	}
	this.ws.onerror = function() {
		console.log("ECLOSED",self)
		self.myremove()
		ohno("Websocket Error! Refresh in a bit, it might have been restarted...")
	}
	this.ws.onclose = function() {
		console.log("CLOSED",self)
		self.myremove()
	}
	this.ws.onmessage = this.onmessage.bind(this)
}
sock.prototype.start = function() {
	this.lastStep = (new Date())
	setInterval(this.step.bind(this),1000/60);
}
sock.prototype.step = function() {
	var actor;
	var now = (new Date())
	var diff = (now - this.lastStep) / 1000
	if (this.room!=null) {
		if (!this.room.step) {
			console.log(this.room)
		} else {
			this.room.step(diff)
		}
	}
	this.lastStep = now
}
sock.prototype.dc = function() {
	console.log("DISCON FROM", this)
	if (currentRoom.server==this) {
		currentRoom=null
		this.room=null
	}
	this.ws.close()
	servers.removeServer(this)
	this.connected = false
}
sock.prototype.remove = function() {
	if (this.room) {
		this.room.remove()
	}
	console.info("REMOVING", this)
	for(var i in this.rooms) {
		servers.removeRoom(this.rooms[i])
	}
	if (servers.servers.indexOf(this)!=-1)
		new sock(this.location)
}
sock.prototype.tick = function() {
	if (this.connected) {
		this.writePing()
		setTimeout(this.mytick,5000);
	}
}
sock.prototype.onmessage = function(m){
	var dv = new DataView(m.data)
	var off = 0
	while (off < dv.byteLength) {
		var olf = off
		if (this.synced) {
			off = this.readMessage(dv, off)
		} else {
			off = this.handleSync(dv, off-1)
			this.synced = true
		}
		// console.log(off-olf)
	}
}
sock.prototype.readMessage = function(dv, off) {
	var t = dv.getUint8(off)
	var h = this.messageHandlers[this.messageMap[t]]
	if (!h) {
		console.log("UNKNOWN TYPE", t, "MAPPED TO", this.messageMap[t])
		return off+1000000
	}
	// console.log("CALLING",h)
	var v = h(dv, off)
	return v
}
sock.prototype.handleSync = function(dv, off) {
	var l = dv.getUint8(off+1)
	this.messageMap = []
	for(var i=0; i<l; i++) {
		this.messageMap[dv.getUint8(off+2+i)] = i
	}
	return off+l+2
}
sock.prototype.handleNewRoom = function(dv, off) {
	var room = {};
	room.id = dv.getInt32(off+1, true)
	var existingRoom = this.rooms[room.id]
	if (existingRoom) {
		console.log("ROOM UPDATE INCOMING")
		room = existingRoom
	}

	room.width = dv.getFloat32(off+5, true)
	room.height = dv.getFloat32(off+9, true)
	room.startmass = dv.getFloat32(off+13, true)
	room.mergetime = dv.getInt32(off+17, true)
	room.sizemultiplier = dv.getFloat32(off+21, true)
	room.speedmultiplier = dv.getFloat32(off+25, true)
	room.playercount = dv.getInt32(off+29, true)

	var len = dv.getInt32(off+33, true)
	console.log("STRLEN",len)
	if (len<100000 && len > -1) {
		var name = dv.getUTF8String(off+37,len)
		room.name = name
	}

	if (!existingRoom) {
		room.server = this
		console.log("NEW ROOM",room)
		this.rooms[room.id] = room
		servers.addRoom(room)
	}


	return off + len + 37
}
sock.prototype.handleNewActor = function(dv, off) {
	var id = dv.getInt32(off+1, true)
	var x = dv.getFloat32(off+5, true)
	var y = dv.getFloat32(off+9, true)
	var mass = dv.getFloat32(off+13, true)
	console.info("CREATING ACTOR",id,"AT",x,y)
	var p = new actor(this.room, id, x, y)
	p.mass = mass
	return off + 17
}
sock.prototype.handleNewPellet = function(dv, off) {
	var x = dv.getInt32(off+1, true)
	var y = dv.getInt32(off+5, true)
	var style = dv.getInt32(off+9, true)
	var t = this.room.findTile(x,y)
	var p = new pellet(t, x, y, style)
	return off + 13
}
sock.prototype.handleRemovePellet = function(dv, off) {
	var dx = dv.getInt32(off+1, true)
	var dy = dv.getInt32(off+5, true)
	var p = this.room.findTile(dx,dy).find(dx,dy)

	if (p) {
		p.remove()
	} else {
		console.log("COULDNT FIND", dx, dy)
	}
	return off + 9
}
sock.prototype.handleNewPlayer = function(dv, off) {
	var id = dv.getInt32(off+1, true)
	var len = dv.getInt32(off+5, true)
	if (len<100000 && len > -1) {
		var name = dv.getUTF8String(off+9,len)
		console.info("CREATING PLAYER", id, "NAME(",len,")",name)
		var p = (new player(this.room, id))
		p.name = name ? name : "";
	} else {
		console.log("INCORRECT LEN", len)
	}
	return off + 9 + len
}
sock.prototype.handleRenamePlayer = function(dv, off) {
	var id = dv.getInt32(off+1, true)
	var len = dv.getInt32(off+5, true)
	if (len<100000 && len > -1) {
		var name = dv.getUTF8String(off+9,len)
		console.log("RENAMING PLAYER", id)
		this.room.players[id].name=name
	} else {
		console.log("INCORRECT LEN", len)
	}
	return off + 9 + len
}
sock.prototype.handleDestroyPlayer = function(dv, off) {
	var id = dv.getInt32(off+1, true)
	console.log("DESTROYING PLAYER", id)
	this.room.players[id].quit()

	return off + 5
}
sock.prototype.handleOwnPlayer = function(dv, off) {
	var id = dv.getInt32(off+1, true)
	console.log("NOW OWNS", id)
	this.room.myplayer = this.room.players[id]
	return off + 5
}
sock.prototype.handleRemoveActor = function(dv, off) {
	var id = dv.getInt32(off+1, true)
	console.info("REMOVING ACTOR", id)
	var p = this.room.actors[id]
	p.remove()
	return off + 5

}
sock.prototype.handleMoveActor = function(dv, off) {
	var id = dv.getInt32(off+1, true)
	var x = dv.getFloat32(off+5, true)
	var y = dv.getFloat32(off+9, true)
	var d = dv.getFloat32(off+13, true)
	var s = dv.getFloat32(off+17, true)

	var p = this.room.actors[id]
	if (!p) {
		console.log("COULDNT FIND",id)
	} else {
		p.setVelocity(s,d)
		p.setPosition(x,y)
	}

	return off + 21
}
sock.prototype.handleSetMass = function(dv, off) {
	var id = dv.getInt32(off+1, true)
	var mass = dv.getFloat32(off+5, true)
	var p = this.room.actors[id]
	p.setmass( mass )
	return off + 9
}
sock.prototype.handleMultiPellet = function(dv, off) {
	var amt = dv.getInt32(off+1, true)
	if (amt<1000000) {
		try {
			for(i in this.room.tiles) {
				this.room.tiles[i].freeadd = false
			}
		}
		catch(e) { console.log(e) }
		console.info("MULTI PELLET", amt)

		var o = off + 5
		for(var i=0;i<amt;i++) {
			var x = dv.getInt32(o, true)
			var y = dv.getInt32(o+4, true)
			var style = dv.getInt32(o+8, true)
			if (Math.random()<.0001) {
				console.info("CREATING PELLET", x, y, style)
			}
			var t = this.room.findTile(x,y)
			var p = new pellet(t, x, y, style)
			o += 12
		}


		try {
			for(i in this.room.tiles) {
				this.room.tiles[i].freeadd = true
			}
		}
		catch(e) { }
	} else {
		console.log("ERROR SIZE", amt)
	}

	return off + 5 + amt * 12
}
sock.prototype.handleDescribeActor = function(dv, off) {
	var t = dv.getUint8(off+1)
	var aid = dv.getInt32(off+2, true)
	switch (t) {
	case 0:
		var pid = dv.getInt32(off+6, true)
		console.info("ACTOR",aid,"IS PLAYERACTOR OWNED BY",pid)
		
		var a = new playeractor(this.room, aid,pid)
		return off + 10
	case 1:
		console.info("ACTOR",aid,"IS VIRUS")
		
		var a = new virus(this.room, aid)
		return off + 6
	case 2:
		console.info("ACTOR",aid,"IS BACTERIA")
		var a = new bacteria(this.room, aid)
		return off + 6
	case 3:
		console.info("ACTOR",aid,"IS BLOB")
	
		var a = new blob(this.room, aid)
		return off + 6
	}
}
sock.prototype.handlePong = function(dv, off) {
	var now = (new Date());
	if (this.latency==0) this.latency = now-this.lastPing
	else this.latency = (this.latency*2+(now-this.lastPing))/3;
	console.info(this, "PING",this.latency,"AFFECTING",this.rooms)
	for(var i in this.rooms) {
		this.rooms[i].refreshElement()
	}
	return off+1
}
sock.prototype.handleStopSpec = function(dv, off) {
	console.log("STOPPING SPECTATOR",this.room.id)
	console.log("NEXT ROOM IS",this.nextRoom)
	console.log("NEXT SERVER IS",this.nextServer)
	this.room.remove()
	if (currentRoom==this.room) {
		currentRoom=null
	}
	this.room = null
	if (this.nextRoom != null) {
		this.nextServer.writeSpectate(this.nextRoom)
		this.nextRoom = null
	}
	return off+1
}

sock.prototype.writeJoin = function(name) {
	var asString = str2ab(name)
	var ab = new ArrayBuffer(1+4+asString.length)
	var dv = new DataView(ab)
	dv.setUint8(0,0,true)
	dv.setInt32(1,asString.length,true)
	for(var i=0;i<asString.length;i+=1) {
		dv.setUint8(5+i, asString[i],true)
	}
	this.ws.send(ab)
}

var mab = new DataView(new ArrayBuffer(13))
mab.setUint8(0,1,true)
sock.prototype.writeMove = function(id,d,s) {
	mab = new DataView(new ArrayBuffer(13))
	mab.setUint8(0,1,true)
	mab.setInt32(1,id,true)
	mab.setFloat32(5,d,true)
	mab.setFloat32(9,s,true)
	this.ws.send(mab)
}
sock.prototype.writeSplit = function() {
	var sab = new DataView(new ArrayBuffer(1))
	sab.setUint8(0,2,true)
	this.ws.send(sab)
}
sock.prototype.writeStopSpectate = function() {
	console.log("REQUESTING TO STOP SPECTATOR",this.room.id)
	var sab = new DataView(new ArrayBuffer(1))
	sab.setUint8(0,3,true)
	this.ws.send(sab)
}
sock.prototype.writeSpectate = function(n) {
	var roomConf = this.rooms[n];
	console.log("ATTEMPTING TO WRITE SPECTATE",n)
	if (currentRoom!=null) {
		console.log("ALREADY SPECTATING",currentRoom.id)
		currentRoom.server.nextRoom = n
		currentRoom.server.nextServer = this
		currentRoom.server.writeStopSpectate()
		return
	}
	this.room = new room(this, roomConf.width,roomConf.height)
	this.room.sizemultiplier = roomConf.sizemultiplier
	this.room.speedmultiplier = roomConf.speedmultiplier 
	this.room.id = n
	currentRoom = this.room

	console.log("SENDING REQUEST TO SPECTATE", n)
	var sab = new DataView(new ArrayBuffer(2))
	sab.setUint8(0,6,true)
	sab.setUint8(1,n,true)
	this.ws.send(sab)
}

sock.prototype.writePing = function() {
	this.lastPing = (new Date())
	var sab = new DataView(new ArrayBuffer(1))
	sab.setUint8(0,4,true)
	this.ws.send(sab)

}

sock.prototype.writeSpit = function() {
	var sab = new DataView(new ArrayBuffer(1))
	sab.setUint8(0,5,true)
	this.ws.send(sab)
}


function ab2str(buf) {
  return String.fromCharCode.apply(null, buf);
}
function str2ab(str) {
	var uintArray = []
	for (var i=0, strLen=str.length; i<strLen; i++) {
		uintArray.push(str.charCodeAt(i));
	}
	return uintArray;
}

function stringToUint(string) {
    var string = btoa(string),
        charList = string.split(''),
        uintArray = [];
    for (var i = 0; i < charList.length; i++) {
        uintArray.push(charList[i].charCodeAt(0));
    }
    console.log("NAME",uintArray)
    return uintArray;
}
function rgb(r, g, b){
  return "rgb("+Math.floor(r)+","+Math.floor(g)+","+Math.floor(b)+")";
}

var canvas = document.getElementById("playarea");
function ohno(x) {
	var errs = document.getElementById("err");
	document.getElementById("mainfloat").style.display="block";
	errs.innerHTML = x;
}
var popup;
window.onload = function() {
	gfx.start()
	if (document.location.protocol.startsWith("https")) {
		servers = new server("wss://"+document.location.hostname+":4000")
	} else {
		servers = new server("ws://"+document.location.hostname+":4000")
	}
	setQuality(maxQuality-1)
	renderBackground = gfx.createRenderBackground(ctx)
	leaderBoardRenderer = gfx.createLeaderBoard({stage:ctx.top})
	window.requestAnimationFrame(render)
	graphicsChanged();
	document.getElementById("loginButton").onclick = function() {
		console.log("JOINING")
		var n = document.getElementById("name").value;
		if (!currentRoom.myplayer) {
			currentRoom.server.writeJoin(n)
		} else {
			document.getElementById("mainfloat").style.display="none";
		}
	}
  document.getElementById("name").onkeypress = function(ev) {
    if(ev.keyCode == 13) {
      document.getElementById("loginButton").onclick();
    }
  }
}

var resFactor = 1;
var ctx;
function graphicsChanged() {
    popup = document.getElementById("mainfloat");
    popup.style.left = ""+String(window.innerWidth/2 - popup.offsetWidth/2)+"px";
    popup.style.top = ""+String(window.innerHeight/2 - popup.offsetHeight/2)+"px";
    canvas.width = window.innerWidth*resFactor;
    canvas.height = window.innerHeight*resFactor;
    canvas.style.width = window.innerWidth;
    canvas.style.height = window.innerHeight;
    canvas.cwidth = window.innerWidth;
    canvas.cheight = window.innerHeight;
    ctx = gfx.getContext(canvas)

    // console.log("New Cam",camera)
}

window.onresize = graphicsChanged;



var camera = {x:0,y:0,width:canvas.width,height:canvas.height,xscale:1,yscale:1};
camera.bbox = function() {
	return [camera.x,camera.y,camera.x+camera.width,camera.y+camera.height]
}

var mousex = canvas.width/2;
var mousey = canvas.height/2;
canvas.onmousemove = function(e) {
	mousex = e.offsetX/canvas.cwidth*canvas.width;
	mousey = e.offsetY/canvas.cheight*canvas.height;
}

var canSplit = true;
var canSpit = true;

document.onkeydown = function(e) {
    e = e || window.event;

	if (canSplit && e.keyCode == '32') {
    	canSplit = false
    	currentRoom.server.writeSplit()
    }
	if (canSpit && e.keyCode == '87') {
    	canSpit = false
    	currentRoom.server.writeSpit()
    }
    if (e.keyCode == '192') {
    	debugMode = true
    }
    if (e.keyCode == '27') {
    	document.getElementById("mainfloat").style.display="block";
    }
    if (debugMode) {
		if (e.keyCode == '68') {
	    	load_graphics_file("dark.js")
	    }
		if (e.keyCode == '69') {
	    	load_graphics_file("tristans.js")
	    }
	}

}
document.onkeyup = function(e) {
    e = e || window.event;
    if (e.keyCode == '32') {
    	canSplit = true

    }
    if (e.keyCode == '87') {
    	canSpit = true

    }
}

function handleScroll(e) {
	console.log(e)
	camPad += e.deltaY/4
	if (camPad>maxCamPad) {
		camPad = maxCamPad
	}
	if (camPad < 10) {
		camPad = 10
	}
}
canvas.addEventListener("mousewheel", handleScroll, false);
canvas.addEventListener("DOMMouseScroll", handleScroll, false);


function load_graphics_file(name) {
	if (gfx_loaded[name]) {
		return
	}
	var js = document.createElement("script");
	js.type = "text/javascript";
	js.src = name;
	document.body.appendChild(js);
	gfx_loaded[name] = true
}

var renderCycles = 0
var graphicsCounts;
var fps = 30;


var renderQuality = -1;
var lastFps = (new Date());
var newFps = 0;

function setQuality(q) {
	if (renderQuality!=q) {
		switch (q) {
		case 0:
			resFactor = 1;
			graphicsChanged();
			break;
		case 1:
			resFactor = 1;
			graphicsChanged();
			break;
		case 2:
			resFactor = 1;
			graphicsChanged();
			break;
		case 3:
			resFactor = 1;
			graphicsChanged();
			break;
		}
	}
	renderQuality=q
}
// var renderArea = gfx.createRenderArea(ctx)
var renderBackground
var leaderBoardRenderer
var skipped = 0
function render() {
	window.requestAnimationFrame(render)
	var now = (new Date())
	if (now-lastFps>1000) {
		lastFps = now
		fps = newFps
		newFps = 0

		// if (fps>55) {
		// 	if (renderQuality < maxQuality) {
		// 		setQuality(renderQuality+1)
		// 	}
		// }
		// if (fps<30 + renderQuality*5) {
		// 	if (renderQuality > 0) {
		// 		setQuality(renderQuality-1)
		// 	}
		// }
	}

	renderCycles -= 1
	if (renderCycles <= 0) {
		if (graphicsCounts) {
			graphicsCounts.tiles /= 100
			graphicsCounts.particles /= 100
			graphicsCounts.tileSkips /= 100
			graphicsCounts.particleSkips /= 100
			graphicsCounts.renderTime /= 100
			graphicsCounts.particleTime /= 100
			graphicsCounts.fps = fps
			if (debugMode) {
				console.log(JSON.stringify(graphicsCounts))
			}
		}
		graphicsCounts = {tiles:0, particles:0, tileSkips:0,
			particleSkips: 0, renderTime:0,
			particleTime: 0}
		renderCycles = 100
	}

	// if (renderQuality<2) {
	if (skipped<1) {
		// console.log("skipping frame")
		skipped += 1
		return;
	}
	skipped = 0
	newFps += 1

	// }
	// renderArea.update(ctx,canvas.width,canvas.height)


	if (currentRoom) {
		var size = currentRoom.getCameraBbox()
		size[0] -= camPad
		size[1] -= camPad
		size[2] += camPad
		size[3] += camPad

		var width = size[2]-size[0]
		var height = size[3]-size[1]
		var ratio = canvas.width/canvas.height
		var haveRatio = width/height
		if (haveRatio<ratio) {
			width = width/haveRatio*ratio
		}
		if (haveRatio>ratio) {
			height = height*haveRatio/ratio
		}
		var midPointX = (size[2]+size[0])/2
		var midPointY = (size[3]+size[1])/2

		camera.x = (midPointX -width/2 + camera.x*3) / 4;
		camera.y = (midPointY -height/2 + camera.y*3) / 4;

		camera.width = (width+camera.width)/2
		camera.height = (height+camera.height)/2

		camera.xscale = canvas.width/camera.width
		camera.yscale = canvas.height/camera.height
		gfx.position(ctx, -camera.x,-camera.y, camera.xscale,camera.yscale)
	}

	if (currentRoom && currentRoom.server) {
		var room = currentRoom;
		var x = camera.x<0 ? 0 : camera.x
		var y = camera.y<0 ? 0 : camera.y
		var w = x + camera.width > room.width ? room.width - x : camera.width
		var h = y + camera.height > room.height ? room.height - y : camera.height

		// var x = camera.x
		// var y = camera.y
		// var w = camera.width
		// var h = camera.height


		renderBackground.update(x, y, w, h)

		currentRoom.render(ctx)
	}


	if (currentRoom && currentRoom.server) {
		draw_leaderboard(ctx,currentRoom)
	}
	gfx.done(ctx)
}

function draw_leaderboard(ctx, room) {
	var playersWithScore = []
	var total = 0
	for(var k in room.players) {
		total += 1
		var p = room.players[k]
		var s = 0
		for(var i in p.owns) {
			var a = p.owns[i];
			s += a.newmass
		}
		if (s!=0) {
			var n = p.name ? p.name : "Microbe"
			playersWithScore.push([n,Math.floor(s)])
		}
	}
	playersWithScore.sort(function(a,b){return b[1]-a[1]})
	leaderBoardRenderer.update(playersWithScore, canvas.width-400, 0, 400, 800, total)
}

// utils

function median(a,b,c) {
	return a<b? b<c? b : a<c? c : a : b<c? a<c? a : c : b; }
