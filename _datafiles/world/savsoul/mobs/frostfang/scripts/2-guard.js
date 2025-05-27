
const PathTargets = [
    [1, 59, 263, 'home'],          // town square => east gate => east residential district => home
    [781, 35, 1, 'home'],          // west residential district => west gate => town square => home
    [166, 63, 76, 62, 61, 'home'], // bank => steelwhisper armory => Hacking Hut => Icy Emporium => Inn => home
    [16, 'home'],                  // south end of beggars lane => home
    [44, 42, 'home'],              // east castle wing => west castle wing => home
];

function onIdle(mob, room) {
    
    if ( mob.PathingAtWaypoint() && mob.IsHome() ) {
        mob.SetAdjective("patrolling", false);
    }

    var random = Math.floor(Math.random() * 10);
    switch (random) {
        case 0:
            mob.Command("emote flexes his muscles");
            return true;
        case 1:
            return true; // does nothing.
        case 2:
            // Start a patrol path
            var randomPath = Math.floor(Math.random() * PathTargets.length);
            var selectedPath = PathTargets[randomPath];
            mob.SetAdjective("patrolling", true);
            mob.Command("pathto "+selectedPath.join(' '));

            return true;
        case 3:
            // wander randomly.
            mob.Command("wander");
            return true;
        default:
            break;
    }

    return false;
}



function onPath(mob, room, eventDetails) {

    if ( eventDetails.status == "waypoint" ) {

        if ( UtilDiceRoll(1, 5) == 1 ) {
            

            if ( modules.follow ) {
            
                followingActors = modules.follow.GetFollowers(mob);
                
                for( var i in followingActors ) {
                    mob.Command("sayto "+followingActors[i].ShorthandId()+" Why are you following me? Leave me be.");
                    mob.Command("follow lose");
                    break;
                }

            }
            

        }
    }

}