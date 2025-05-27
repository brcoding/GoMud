
var SAID_STUFF = false; // Whether lakework has spoken this "visit" to the crash site

const HOME_ROOM_ID = 362;  // old dock
const TRASH_PICKUP_SPOTS = [331,327,323,322]; // miscellaneous points along the way to do a random emote
const CRASH_ROOM_ID = 319; // crashing waves leading to the rocky island

const boatNouns = ["boats", "oars", "ships", "paddles"];
const crashNouns = ["rocks", "crash", "choppy", "water", "waves"];

// Emotes randomly selected at various waypoints.
const randomEmotes = [
    "emote picks up some trash along the shoreline.",
    "emote moves some driftwood to a pile.",
    "emote picks up a small rock and throws it into the lake.",
    "emote gently brushes leaves off a bench.",
    "emote tosses a fallen branch off the trail.",
    "emote stoops to examine a patch of wildflowers.",
    "emote skips a stone across the water.",
    "emote watches a dragonfly hover near the reeds.",
    "emote nudges a small frog back toward the water.",
    "emote smooths out a scuffed trail marker.",
    "emote pauses to listen to the rustling trees.",
    "emote adjusts a loose rock on the path.",
];

function onAsk(mob, room, eventDetails) {

    var roomId = room.RoomId();

    var match = UtilFindMatchIn(eventDetails.askText, boatNouns);
    if ( match.found ) {

        if ( roomId == CRASH_ROOM_ID ) {
            
            mob.Command("say I hit those rocks just over there and lost all of our oars.");
            mob.Command("emote points to the northwest.");

        } else {
            if ( WALK_POSITION >= 5 ) {
                mob.Command("say We lost the oars to the boats when the choppy water caused me to crash against some rocks to the west of here.");
            } else {
                mob.Command("say We lost the oars to the boats when the choppy water caused me to crash against some rocks in the southwest part of the lake.");
            }
        }
        return;
    }

    match = UtilFindMatchIn(eventDetails.askText, crashNouns);
    if ( match.found ) {


        if ( roomId == CRASH_ROOM_ID ) {
            
            mob.Command("say I hit those rocks just over there and lost all of our oars.");
            mob.Command("emote points to the northwest.");

        } else {
            if ( WALK_POSITION >= 5 ) {
                mob.Command("say Just a little west of here are some rocky islands.");
                if ( WALK_DIRECTION > 0 ) {
                    mob.Command("say I'm headed there now to see if there's any trash washed ashore.");
                }else {
                    mob.Command("say I'm just coming back from cleaning up trash from there.");
                }
            } else {
                mob.Command("say In the southwest part of the lake are some rocky islands.");
                mob.Command("say I visit there every so often to clean up trash that washes ashore.");

                if ( WALK_DIRECTION > 0 ) {
                    mob.Command("say I'm heading there soon to see if there's any trash washed ashore.");
                }else {
                    mob.Command("say I'm just coming back from cleaning up trash from there.");
                }
            }
        }
        return;

    }

}


function onPath(mob, room, eventDetails) {

    if ( eventDetails.status == "waypoint" && mob.GetRoomId() != CRASH_ROOM_ID ) {
    
        if ( UtilDiceRoll(1, 5) == 1 ) {
            var emoteSelection = UtilDiceRoll(1, randomEmotes.length)-1;
            mob.Command(randomEmotes[emoteSelection]);
        }    

    }

}

function onGive(mob, room, eventDetails) {

    if (eventDetails.item) {
        if (eventDetails.item.ItemId != 10016) {
            mob.Command("look !"+String(eventDetails.item.ItemId));
            mob.Command("drop !"+String(eventDetails.item.ItemId), UtilGetSecondsToTurns(5));
            return true;
        }

        mob.Command("say Thanks, but my days of rowing are over. I'm just a lowly lake worker now.");
        mob.Command("drop !"+String(eventDetails.item.ItemId));
        return true;
    }

}


// Invoked once every round if mob is idle
function onIdle(mob, room) {

    if ( mob.GetRoomId() == CRASH_ROOM_ID ) {

        if ( !SAID_STUFF ) {
            mob.Command("emote squints and peers towards a rocky island in the lake to the northwest.");
            mob.Command("emote mutters to himself.");
            mob.Command("say Ever since I crashed my boat on those rocks, I've been demoted to cleaning up the lakeshore.", 2);

            SAID_STUFF = true;
            return true;
        }
    }

    if ( UtilDiceRoll(1, 2) > 1 ) {
        return true;
    }

    if ( mob.IsHome() ) { 
        SAID_STUFF = false; // reset once they get home
        mob.Command("pathto " + TRASH_PICKUP_SPOTS.join(" ") + " " + String(CRASH_ROOM_ID));
        return true;
    }

    if ( mob.GetRoomId() == CRASH_ROOM_ID ) {
        mob.Command("pathto " + TRASH_PICKUP_SPOTS.slice().reverse().join(" ") + " home");
    }

    return true;
}

