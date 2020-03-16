#!/bin/bash
HN=`hostname`
AV=`curl https://www.onoffmix.com/event/198749  |grep number_txt |grep available |awk '{if(NR==2) print $4}' |sed 's/class="number_txt">//g' |sed 's/span//g'`
echo $HN : $AV
