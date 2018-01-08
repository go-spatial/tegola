//	WebHelp 5.10.005
var gsPPath="";
var gaPaths=new Array();
var gaAvenues=new Array();

var goFrame=null;
var gsStartPage="";
var gsRelCurPagePath="";
var gsSearchFormHref="";
var gnTopicOnly=-1;
var gnOutmostTopic=-1;

var BTN_TEXT=1;
var BTN_IMG=2;

var goSync=null;

var goShow=null;
var goHide=null;

var goPrev=null;
var goNext=null;
var gnForm=0;
var goShowNav=null;
var goHideNav=null;

var goWebSearch=null;

var gsBtnStyle="";
var gaButtons=new Array();
var gaTypes=new Array();
var whtopic_foldUnload=null;
var gbWhTopic=false;
var gbCheckSync=false;
var gbSyncEnabled=false;

function setButtonFont(sType,sFontName,sFontSize,sFontColor,sFontStyle,sFontWeight,sFontDecoration)
{
	var vFont=new whFont(sFontName,sFontSize,sFontColor,sFontStyle,sFontWeight,sFontDecoration);
	gsBtnStyle+=".whtbtn"+sType+"{"+getFontStyle(vFont)+"}";
}

function writeBtnStyle()
{
	if(gaButtons.length>0)
	{
		if(gsBtnStyle.length>0)
		{
			var sStyle="<style type='text/css'>";
			sStyle+=gsBtnStyle+"</style>";
			document.write(sStyle);
		}
	}
}

function button(sText,nWidth,nHeight)
{
	this.sText=sText;
	this.nWidth=nWidth;
	this.nHeight=nHeight;
	
	this.aImgs=new Array();
	var i=0;
	while(button.arguments.length>i+3)
	{
		this.aImgs[i]=button.arguments[3+i];
		i++;
	}
}

//project info
function setRelStartPage(sPath)
{
	if(gsPPath.length==0)
	{
		gsPPath=_getFullPath(_getPath(document.location.href),_getPath(sPath));
		gsStartPage=_getFullPath(_getPath(document.location.href),sPath);
		gsRelCurPagePath=_getRelativeFileName(gsStartPage,document.location.href);
	}
}

function getImage(oImage,sType)
{
	var sImg="";
	if(oImage&&oImage.aImgs&&(oImage.aImgs.length>0))
	{
		sImg+="<img alt=\""+sType+"\" src=\""+oImage.aImgs[0]+"\"";
		if(oImage.nWidth>0)
			sImg+=" width="+oImage.nWidth;
		if(oImage.nHeight>0)
			sImg+=" height="+oImage.nHeight;
		sImg+=" border=0>";
	}
	return sImg;
}

function addTocInfo(sTocPath)
{
	gaPaths[gaPaths.length]=sTocPath;
}

function addAvenueInfo(sName,sPrev,sNext)
{
	gaAvenues[gaAvenues.length]=new avenueInfo(sName,sPrev,sNext);	
}

function addButton(sType,nStyle,sText,sHref,sOnClick,sOnMouseOver,sOnLoad,nWidth,nHeight,sImg1,sImg2,sImg3)
{
	var sButton="";
	var nBtn=gaButtons.length;
	if(sType=="prev")
	{
		if(canGo(false))
		{
			var sTitle="Previous Topic";
			goPrev=new button(sText,nWidth,nHeight,sImg1,sImg2,sImg3);
			sButton="<a title=\""+sTitle+"\" class=\"whtbtnprev\" href=\"javascript:void(0);\" onclick=\"goAvenue(false);return false;\">";
			if(nStyle==BTN_TEXT)
				sButton+=goPrev.sText;
			else
				sButton+=getImage(goPrev,sTitle);
			sButton+="</a>";
		}
	}
	else if(sType=="next")
	{
		if(canGo(true))
		{
			var sTitle="Next Topic";
			goNext=new button(sText,nWidth,nHeight,sImg1,sImg2,sImg3);
			sButton="<a title=\""+sTitle+"\" class=\"whtbtnnext\" href=\"javascript:void(0);\" onclick=\"goAvenue(true);return false;\">";
			if(nStyle==BTN_TEXT)
				sButton+=goNext.sText;
			else
				sButton+=getImage(goNext,sTitle);
			sButton+="</a>";
		}
	}
	else if(sType=="show")
	{
		if(isTopicOnly()&&(!gbOpera6||gbOpera7))
		{
			var sTitle="Show Navigation Component";
			goShow=new button(sText,nWidth,nHeight,sImg1,sImg2,sImg3);
			sButton="<a title=\""+sTitle+"\" class=\"whtbtnshow\" href=\"javascript:void(0);\" onclick=\"show();return false;\">";
			if(nStyle==BTN_TEXT)
				sButton+=goShow.sText;
			else
				sButton+=getImage(goShow,sTitle);
			sButton+="</a>";
		}
	}
	else if(sType=="hide")
	{
		if(!isTopicOnly()&&!gbOpera6)
		{
			var sTitle="Hide Navigation Component";
			goHide=new button(sText,nWidth,nHeight,sImg1,sImg2,sImg3);
			sButton="<a title=\""+sTitle+"\" class=\"whtbtnhide\" href=\"javascript:void(0);\" onclick=\"hide();return false;\">";
			if(nStyle==BTN_TEXT)
				sButton+=goHide.sText;
			else
				sButton+=getImage(goHide,sTitle);
			sButton+="</a>";
		}
	}
	else if(sType=="shownav")
	{
		if(isShowHideEnable())
		{
			var sTitle="Show Navigation Component";
			goShowNav=new button(sText,nWidth,nHeight,sImg1,sImg2,sImg3);
			sButton="<a title=\""+sTitle+"\" class=\"whtbtnshownav\" href=\"javascript:void(0);\" onclick=\"showHidePane(true);return false;\">";
			if(nStyle==BTN_TEXT)
				sButton+=goShowNav.sText;
			else
				sButton+=getImage(goShowNav,sTitle);
			sButton+="</a>";
		}
	}
	else if(sType=="hidenav")
	{
		if(isShowHideEnable())
		{
			var sTitle="Hide Navigation Component";
			goHideNav=new button(sText,nWidth,nHeight,sImg1,sImg2,sImg3);
			sButton="<a title=\""+sTitle+"\" class=\"whtbtnhidenav\" href=\"javascript:void(0);\" onclick=\"showHidePane(false);return false;\">";
			if(nStyle==BTN_TEXT)
				sButton+=goHideNav.sText;
			else
				sButton+=getImage(goHideNav,sTitle);
			sButton+="</a>";
		}
	}
	else if(sType=="synctoc")
	{
		if(gaPaths.length>0)
		{
			var sTitle="Sync TOC";
			goSync=new button(sText,nWidth,nHeight,sImg1,sImg2,sImg3);
			sButton="<a title=\""+sTitle+"\" class=\"whtbtnsynctoc\" href=\"javascript:void(0);\" onclick=\"syncWithShow();return false;\">";
			if(nStyle==BTN_TEXT)
				sButton+=goSync.sText;
			else
				sButton+=getImage(goSync,sTitle);
			sButton+="</a>";
		}
	}
	else if(sType=="websearch")
	{
		if(gsSearchFormHref.length>0)
		{
			var sTitle="WebSearch";
			goWebSearch=new button(sText,nWidth,nHeight,sImg1,sImg2,sImg3);
			sButton="<a title=\""+sTitle+"\" class=\"whtbtnwebsearch\" href=\""+gsSearchFormHref+"\">";
			if(nStyle==BTN_TEXT)
				sButton+=goWebSearch.sText;
			else
				sButton+=getImage(goWebSearch,sTitle);
			sButton+="</a>";
		}
	}
	else if(sType=="searchform")
	{
		gaButtons[nBtn]="NeedSearchForm";
		gaTypes[nBtn]=sType;
	}
	if(sButton.length!=0)
	{
		if(nStyle==BTN_TEXT)
			sButton+="&nbsp;";
		gaButtons[nBtn]="<td>"+sButton+"</td>";
		gaTypes[nBtn]=sType;
	}
}

function isSyncEnabled()
{
	if(!gbCheckSync)
	{
		var oMsg=new whMessage(WH_MSG_ISSYNCSSUPPORT,this,1,null);
		if(SendMessage(oMsg))
		{
			gbSyncEnabled=oMsg.oParam;
		}
		gbCheckSync=true;
	}
	return gbSyncEnabled;
}

function isInPopup()
{
	return (window.name.indexOf("BSSCPopup")!=-1);
}

function getIntopicBar(sAlign)
{
	var sHTML="";
	if(gaButtons.length>0)
	{
		sHTML+="<div align="+sAlign+">";

		sHTML+="<table cellpadding=\"2\" cellspacing=\"0\" border=\"0\"><tr>";
		for(var i=0;i<gaButtons.length;i++)
		{
			if(gaTypes[i]!="synctoc"||isSyncEnabled())
			{
				if(gaButtons[i]=="NeedSearchForm")
					sHTML+=getSearchFormHTML();
				else
					sHTML+=gaButtons[i];
			}
		}
		sHTML+="</tr></table>";

		sHTML+="</div>";
	}
	return sHTML;
}


function writeIntopicBar(nAligns)
{
	if(isInPopup()) return;
	if(gaButtons.length>0)
	{
		var sHTML="";
		if(nAligns!=0)
		{
			sHTML+="<table width=100%><tr>"
			if(nAligns&1)
				sHTML+="<td width=33%>"+getIntopicBar("left")+"</td>";
			if(nAligns&2)
				sHTML+="<td width=34%>"+getIntopicBar("center")+"</td>";
			if(nAligns&4)
				sHTML+="<td width=33%>"+getIntopicBar("right")+"</td>";
			sHTML+="</tr></table>";
			document.write(sHTML);
		}
	}
}

function sendAveInfoOut()
{
	if(!isInPopup())
		setTimeout("sendAveInfo();",100);
}

function sendAveInfo()
{
	var oMsg=new whMessage(WH_MSG_AVENUEINFO,this,1,gaAvenues);
	SendMessage(oMsg);
}


function onNext()
{
	var oMsg=new whMessage(WH_MSG_NEXT,this,1,null);
	SendMessage(oMsg);
}

function onPrev()
{
	var oMsg=new whMessage(WH_MSG_PREV,this,1,null);
	SendMessage(oMsg);
}

function createSyncInfo()
{
	var oParam=new Object();
	if(gsPPath.length==0)
		gsPPath=_getPath(document.location.href);
	oParam.sPPath=gsPPath;
	oParam.sTPath=document.location.href;
	oParam.aPaths=gaPaths;
	return oParam;
}

function syncWithShow()
{
	if(isTopicOnly())
		show();
	else
	{
		sync();
		showTocPane();
	}
}

function showTocPane()
{
	var oMsg=new whMessage(WH_MSG_SHOWTOC,this,1,null);
	SendMessage(oMsg);
}

function sendSyncInfo()
{
	if(!isInPopup())
	{
		var oParam=null;
		if(gaPaths.length>0)
		{
			oParam=createSyncInfo();
		}
		var oMsg=new whMessage(WH_MSG_SYNCINFO,this,1,oParam);
		SendMessage(oMsg);
	}
}

function sendInvalidSyncInfo()
{
	if(!isInPopup())
	{
		var oMsg=new whMessage(WH_MSG_SYNCINFO,this,1,null);
		SendMessage(oMsg);
	}
}

function enableWebSearch(bEnable)
{
	if(!isInPopup())
	{
		var oMsg=new whMessage(WH_MSG_ENABLEWEBSEARCH,this,1,bEnable);
		SendMessage(oMsg);
	}
}

function autoSync(nSync)
{
	if(nSync==0) return;
	if(isInPopup()) return;
	if(isOutMostTopic())
		sync();
}

function isOutMostTopic()
{
	if(gnOutmostTopic==-1)
	{
		var oMessage=new whMessage(WH_MSG_ISINFRAMESET,this,1,null);
		if(SendMessage(oMessage))
			gnOutmostTopic=0;
		else
			gnOutmostTopic=1;
	}
	return (gnOutmostTopic==1);
}

function sync()
{
	if(gaPaths.length>0)
	{
		var oParam=createSyncInfo();
		var oMessage=new whMessage(WH_MSG_SYNCTOC,this,1,oParam);
		SendMessage(oMessage);
	}
}


function avenueInfo(sName,sPrev,sNext)
{
	this.sName=sName;
	this.sPrev=sPrev;
	this.sNext=sNext;
}

function getCurrentAvenue()
{
	var oParam=new Object();
	oParam.sAvenue=null;
	var oMessage=new whMessage(WH_MSG_GETCURRENTAVENUE,this,1,oParam);
	SendMessage(oMessage);
	return oParam.sAvenue;
}

function unRegisterListener()
{
	sendInvalidSyncInfo();
	enableWebSearch(false);
	if(whtopic_foldUnload)
		whtopic_foldUnload();
}

function onSendMessage(oMsg)
{
	var nMsgId=oMsg.nMessageId;
	if(nMsgId==WH_MSG_GETAVIAVENUES)
	{
		oMsg.oParam.aAvenues=gaAvenues;
		return false;
	}
	else if(nMsgId==WH_MSG_GETTOCPATHS)
	{
		if(isOutMostTopic())
		{
			oMsg.oParam.oTocInfo=createSyncInfo();
			return false;		
		}
		else
			return true;
	}
	else if(nMsgId==WH_MSG_NEXT)
	{
		goAvenue(true);
	}
	else if(nMsgId==WH_MSG_PREV)
	{
		goAvenue(false);
	}
	else if(nMsgId==WH_MSG_WEBSEARCH)
	{
		websearch();
	}
	return true;
}

function goAvenue(bNext)
{
	var sTopic=null;
	var sAvenue=getCurrentAvenue();
	var nAvenue=-1;
	if(sAvenue!=null&&sAvenue!="")
	{
		for(var i=0;i<gaAvenues.length;i++)
		{
			if(gaAvenues[i].sName==sAvenue)
			{
				nAvenue=i;
				break;
			}
		}
		if(nAvenue!=-1)
		{
			if(bNext)
				sTopic=gaAvenues[nAvenue].sNext;
			else
				sTopic=gaAvenues[nAvenue].sPrev;
		}
	}
	else
	{
		for(var i=0;i<gaAvenues.length;i++)
		{
			if(gaAvenues[i].sNext!=null&&gaAvenues[i].sNext.length>0&&bNext)
			{
				sTopic=gaAvenues[i].sNext;
				break;
			}
			else if(gaAvenues[i].sPrev!=null&&gaAvenues[i].sPrev.length>0&&!bNext)
			{
				sTopic=gaAvenues[i].sPrev;
				break;
			}
		}
	}
	
	if(sTopic!=null&&sTopic!="")
	{
		if(gsPPath!=null&&gsPPath!="")
		{
			sFullTopicPath=_getFullPath(gsPPath,sTopic);
			document.location=sFullTopicPath;
		}
	}
}

function canGo(bNext)
{
	for(var i=0;i<gaAvenues.length;i++)
	{
		if((gaAvenues[i].sNext!=null&&gaAvenues[i].sNext.length>0&&bNext)||
			(gaAvenues[i].sPrev!=null&&gaAvenues[i].sPrev.length>0&&!bNext))
			return true;
	}
	return false;
}

function show()
{
	if(gsStartPage!="")
		window.location=gsStartPage+"#"+gsRelCurPagePath;
}

function hide()
{
	if(goFrame!=null)
	{
		goFrame.location=window.location;
	}
}

function isTopicOnly()
{
	if(gnTopicOnly==-1)
	{
		var oParam=new Object();
		oParam.oFrame=null;
		var oMsg=new whMessage(WH_MSG_GETSTARTFRAME,this,1,oParam);
		if(SendMessage(oMsg))
		{
			goFrame=oParam.oFrame;
			gnTopicOnly=0;
		}
		else
			gnTopicOnly=1;
	}
	if(gnTopicOnly==1)
		return true;
	else
		return false;
}

function websearch()
{
	if(gbNav4)
	{
		if(document.ehelpform)
			document.ehelpform.submit();
	}
	else
	{
		if(window.ehelpform)
			window.ehelpform.submit();
	}
}

function addSearchFormHref(sHref)
{
	gsSearchFormHref=sHref;
	enableWebSearch(true);
}

function searchB(nForm)
{
	var sValue=eval("document.searchForm"+nForm+".searchString.value");
	var oMsg=new whMessage(WH_MSG_SEARCHTHIS,this,1,sValue);
	SendMessage(oMsg);
}

function getSearchFormHTML()
{
	var sHTML="";
	gnForm++;
	var sFormName="searchForm"+gnForm;
	var sButton="<form name=\""+sFormName+"\" method=\"POST\" action=\"javascript:searchB("+gnForm+")\">"
	sButton+="<input type=\"text\" name=\"searchString\" value=\"- Full Text search -\" size=\"20\"/>";
	if(""=="text")
	{
		sButton+="<a class=\"searchbtn\" href=\"javascript:void(0);\" onclick=\""+sFormName+".submit();return false;\"></a>";
	}
	else if(""=="image")
	{
		sButton+="<a class=\"searchbtn\" href=\"javascript:void(0);\" onclick=\""+sFormName+".submit();return false;\">"
		sButton+="<img src=\"\" border=0></a>";
	}
	sButton+="</form>";
	sHTML="<td align=\"center\">"+sButton+"</td>";
	return sHTML;
}

function showHidePane(bShow)
{
	var oMsg=null;
	if(bShow)
		oMsg=new whMessage(WH_MSG_SHOWPANE,this,1,null);
	else
		oMsg=new whMessage(WH_MSG_HIDEPANE,this,1,null);
	SendMessage(oMsg);
}

function isShowHideEnable()
{
	if(gbIE4)
		return true;
	else
		return false;
}


function PickupDialog_Invoke()
{
	if(!gbIE4||gbMac)
	{
		if(typeof(_PopupMenu_Invoke)=="function")
			return _PopupMenu_Invoke(PickupDialog_Invoke.arguments);
	}
	else
	{
		if(PickupDialog_Invoke.arguments.length>2)
		{
			var sPickup="whskin_pickup.htm";
			var sPickupPath=gsPPath+sPickup;
			if(gbIE4)
			{
				var sFrame=PickupDialog_Invoke.arguments[1];
				var aTopics=new Array();
				for(var i=2;i<PickupDialog_Invoke.arguments.length;i+=2)
				{
					var j=aTopics.length;
					aTopics[j]=new Object();
					aTopics[j].m_sName=PickupDialog_Invoke.arguments[i];
					aTopics[j].m_sURL=PickupDialog_Invoke.arguments[i+1];
				}

				if(aTopics.length>1)
				{
					var nWidth=300;
					var nHeight=180;
					var	nScreenWidth=screen.width;
					var	nScreenHeight=screen.height;
					var nLeft=(nScreenWidth-nWidth)/2;
					var nTop=(nScreenHeight-nHeight)/2;
					if(gbIE4)
					{
						var vRet=window.showModalDialog(sPickupPath,aTopics,"dialogHeight:"+nHeight+"px;dialogWidth:"+nWidth+"px;resizable:yes;status:no;scroll:no;help:no;center:yes;");
						if(vRet)
						{
							var sURL=vRet.m_url;
							if(sFrame)
								window.open(sURL,sFrame);
							else
								window.open(sURL,"_self");
						}
					}
				}
				else if(aTopics.length==1)
				{
					var sURL=aTopics[0].m_sURL
					if(sFrame)
						window.open(sURL,sFrame);
					else
						window.open(sURL,"_self");
				}
			}
		}
	}
}

if(window.gbWhUtil&&window.gbWhMsg&&window.gbWhVer&&window.gbWhProxy)
{
	RegisterListener("bsscright",WH_MSG_GETAVIAVENUES);
	RegisterListener("bsscright",WH_MSG_GETTOCPATHS);
	RegisterListener("bsscright",WH_MSG_NEXT);
	RegisterListener("bsscright",WH_MSG_PREV);
	RegisterListener("bsscright",WH_MSG_WEBSEARCH);
	if(gbMac&&gbIE4)
	{
		if(typeof(window.onunload)!="unknown")
			if(window.onunload.toString!=unRegisterListener.toString)
				whtopic_foldUnload=window.onunload;
	}
	else
	{
		if(window.onunload)
			if(window.onunload.toString!=unRegisterListener.toString)
				whtopic_foldUnload=window.onunload;
	}
	window.onunload=unRegisterListener;
	
	gbWhTopic=true;
}
else
	document.location.reload();